// Package agentless collects deep inventory from remote hosts WITHOUT an
// installed agent, using credentials. The Windows collector uses WinRM/WS-Man
// (PowerShell remoting) — proven against workgroup machines with a local admin.
//
// It runs ONE PowerShell script that emits a single JSON blob shaped as the
// model.EndpointReport wire format (top-level metrics + a nested "hardware"
// object), so an agentless-discovered machine flows through the SAME
// ingest/dashboard path as an agent-reported one. Hardware stays raw JSON.
//
// Notes (from research):
//   - masterzen/winrm has built-in NTLM via ClientNTLM{} (no external lib).
//   - NTLM is NOT goroutine-safe on a shared client: one client per host. We
//     create a fresh client per scan, so this is satisfied.
//   - Workgroup local admin needs LocalAccountTokenFilterPolicy=1 on target.
package agentless

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fullstackit/watchclaw/control-plane/internal/model"
	"github.com/masterzen/winrm"
)

// inventoryPS is the single read-only inventory script. Each section is wrapped
// so a missing cmdlet (older Windows) yields null instead of aborting the run.
// Output keys match the EndpointReport wire format + hardware sub-keys the
// dashboard already renders (services/startup_items/connections/software).
const inventoryPS = `
$ErrorActionPreference='Continue'; $ProgressPreference='SilentlyContinue'
function S($b){ try{ & $b }catch{ $null } }
$os=S{Get-CimInstance Win32_OperatingSystem}; $cs=S{Get-CimInstance Win32_ComputerSystem}
$bios=S{Get-CimInstance Win32_BIOS}; $bb=S{Get-CimInstance Win32_BaseBoard}
$cpu=S{Get-CimInstance Win32_Processor | Select-Object -First 1}
$memTotalMB=if($cs){[uint64][math]::Round($cs.TotalPhysicalMemory/1MB)}else{0}
$memFreeMB=if($os){[uint64][math]::Round($os.FreePhysicalMemory/1KB)}else{0}
$memUsedMB=[uint64]([math]::Max(0,$memTotalMB-$memFreeMB))
$load=S{ (Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average }
function SW {
 $r=@(); $p=@('HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*','HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*')
 foreach($k in $p){ S{ Get-ItemProperty $k -EA SilentlyContinue | Where-Object {$_.DisplayName} | ForEach-Object {
   $r+=[ordered]@{name=$_.DisplayName; version=[string]$_.DisplayVersion; publisher=[string]$_.Publisher} } } }
 ,$r }
$disks=@(S{ Get-CimInstance Win32_LogicalDisk -Filter 'DriveType=3' | ForEach-Object {
  $t=[math]::Round($_.Size/1GB,2); $f=[math]::Round($_.FreeSpace/1GB,2)
  [ordered]@{mount=$_.DeviceID; total_gb=$t; used_gb=[math]::Round($t-$f,2); usage_pct=if($t){[math]::Round(($t-$f)/$t*100,1)}else{0}} })
$svc=@(S{ Get-CimInstance Win32_Service | Where-Object {$_.StartMode -eq 'Auto'} | ForEach-Object {
  [ordered]@{name=$_.Name; display_name=$_.DisplayName; state=$_.State; start_mode=$_.StartMode; path=[string]$_.PathName} })
$start=@(); 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run','HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run' | ForEach-Object {
  $loc=$_; $kk=S{Get-ItemProperty $_ -EA SilentlyContinue}; if($kk){ $kk.PSObject.Properties | Where-Object {$_.Name -notlike 'PS*'} | ForEach-Object {
    $start+=[ordered]@{name=$_.Name; command=[string]$_.Value; location=$loc} } } }
$conns=@(S{ Get-NetTCPConnection -State Listen,Established -EA SilentlyContinue | ForEach-Object {
  $pn=S{ (Get-Process -Id $_.OwningProcess -EA SilentlyContinue).ProcessName }
  [ordered]@{local=("{0}:{1}" -f $_.LocalAddress,$_.LocalPort); remote=("{0}:{1}" -f $_.RemoteAddress,$_.RemotePort); state=$_.State.ToString(); pid=[int]$_.OwningProcess; process=[string]$pn} })
$hot=@(S{ Get-HotFix | ForEach-Object {[string]$_.HotFixID} })
[ordered]@{
 hostname=if($cs){$cs.Name}else{$env:COMPUTERNAME}
 os='Windows'; os_version=if($os){$os.Version}else{''}; kernel_version=if($os){$os.Caption}else{''}
 cpu_cores=if($cpu){[int]$cpu.NumberOfLogicalProcessors}else{0}
 cpu_usage_pct=if($load){[double]$load}else{0}
 mem_total_mb=$memTotalMB; mem_used_mb=$memUsedMB; mem_usage_pct=if($memTotalMB){[math]::Round($memUsedMB/$memTotalMB*100,1)}else{0}
 uptime_seconds=if($os){[int]((Get-Date)-$os.LastBootUpTime).TotalSeconds}else{0}
 disks=$disks
 hardware=[ordered]@{
   system=[ordered]@{manufacturer=if($cs){[string]$cs.Manufacturer}else{''}; model=if($cs){[string]$cs.Model}else{''}; bios_serial=if($bios){[string]$bios.SerialNumber}else{''}; baseboard_serial=if($bb){[string]$bb.SerialNumber}else{''}}
   software=SW; hotfixes=$hot; services=$svc; startup_items=$start; connections=$conns; source='agentless-winrm'
 }
} | ConvertTo-Json -Depth 6 -Compress
`

// ScanWindows connects to a remote Windows host over WinRM and returns a parsed
// EndpointReport. port=5985 (http) or 5986 (https+insecure for self-signed);
// user/pass is a local admin (workgroup) or domain user.
func ScanWindows(ctx context.Context, host string, port int, https, insecure bool, user, pass string) (model.EndpointReport, error) {
	var zero model.EndpointReport
	ep := winrm.NewEndpoint(host, port, https, insecure, nil, nil, nil, 30*time.Second)
	params := winrm.DefaultParameters
	params.TransportDecorator = func() winrm.Transporter { return &winrm.ClientNTLM{} }
	params.Timeout = "PT180S"
	params.EnvelopeSize = 8 * 1024 * 1024

	client, err := winrm.NewClientWithParameters(ep, user, pass, params)
	if err != nil {
		return zero, fmt.Errorf("winrm client: %w", err)
	}
	stdout, stderr, code, err := client.RunWithContextWithString(ctx, winrm.Powershell(inventoryPS), "")
	if err != nil {
		return zero, classify(err, stderr)
	}
	if code != 0 {
		return zero, fmt.Errorf("inventory script exit %d: %s", code, strings.TrimSpace(stderr))
	}
	return parseWinReport([]byte(stdout))
}

// parseWinReport unmarshals the PowerShell JSON straight into an EndpointReport
// (hardware stays raw JSON). Exported shape is the same as the Rust agent's.
func parseWinReport(stdout []byte) (model.EndpointReport, error) {
	var rep model.EndpointReport
	trimmed := strings.TrimSpace(string(stdout))
	if trimmed == "" {
		return rep, fmt.Errorf("empty inventory output")
	}
	if err := json.Unmarshal([]byte(trimmed), &rep); err != nil {
		return rep, fmt.Errorf("parse inventory json: %w (got %.120q)", err, trimmed)
	}
	if rep.Hostname == "" {
		return rep, fmt.Errorf("inventory missing hostname")
	}
	rep.CollectedAt = time.Now().UTC().Format(time.RFC3339)
	return rep, nil
}

// classify turns opaque WinRM errors into actionable messages (research §3).
func classify(err error, stderr string) error {
	s := err.Error() + " " + stderr
	switch {
	case strings.Contains(s, "0x80070005") || strings.Contains(s, "AccessDenied") || strings.Contains(s, "401"):
		return fmt.Errorf("access denied — set LocalAccountTokenFilterPolicy=1 on target or use built-in Administrator: %w", err)
	case strings.Contains(s, "0x800706BA") || strings.Contains(s, "connection refused") || strings.Contains(s, "connectex"):
		return fmt.Errorf("WinRM unreachable — service off or firewall (run 'winrm quickconfig', open 5985/5986): %w", err)
	case strings.Contains(s, "x509") || strings.Contains(s, "certificate"):
		return fmt.Errorf("TLS cert not trusted — use insecure=true or pin CA for self-signed 5986: %w", err)
	default:
		return fmt.Errorf("winrm scan: %w", err)
	}
}
