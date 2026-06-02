package agentless

import (
	"encoding/json"
	"testing"
)

// Sample = the exact wire shape our PowerShell emits (EndpointReport + nested
// hardware). Verifies parseWinReport maps it without touching a real host.
const sample = `{
 "hostname":"DESKTOP-X","os":"Windows","os_version":"10.0.26100","kernel_version":"Windows 11 Pro",
 "cpu_cores":16,"cpu_usage_pct":12.5,"mem_total_mb":32768,"mem_used_mb":19000,"mem_usage_pct":58,
 "uptime_seconds":99000,
 "disks":[{"mount":"C:","total_gb":2000,"used_gb":1220,"usage_pct":61}],
 "hardware":{
   "system":{"manufacturer":"Asus","model":"ROG","bios_serial":"N2NRKD","baseboard_serial":"BB1"},
   "software":[{"name":"Git","version":"2.54","publisher":"GitHub"}],
   "hotfixes":["KB5089569"],
   "services":[{"name":"AnyDesk","display_name":"AnyDesk","state":"Running","start_mode":"Auto","path":"C:\\anydesk.exe"}],
   "startup_items":[{"name":"SecHealth","command":"sechealth.exe","location":"HKLM:\\...\\Run"}],
   "connections":[{"local":"192.168.1.20:5001","remote":"192.168.1.10:445","state":"Established","pid":4,"process":"System"}],
   "source":"agentless-winrm"
 }
}`

func TestParseWinReport(t *testing.T) {
	rep, err := parseWinReport([]byte(sample))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if rep.Hostname != "DESKTOP-X" {
		t.Errorf("hostname=%q", rep.Hostname)
	}
	if rep.OS != "Windows" || rep.OSVersion != "10.0.26100" {
		t.Errorf("os=%q ver=%q", rep.OS, rep.OSVersion)
	}
	if rep.CPUCores != 16 || rep.MemUsagePct != 58 {
		t.Errorf("cpu_cores=%d mem%%=%v", rep.CPUCores, rep.MemUsagePct)
	}
	if len(rep.Disks) != 1 || rep.Disks[0].UsagePct != 61 {
		t.Errorf("disks=%+v", rep.Disks)
	}
	if rep.CollectedAt == "" {
		t.Error("collected_at not set")
	}
	// hardware stays raw JSON; confirm the dashboard-rendered keys survived.
	var hw map[string]json.RawMessage
	if err := json.Unmarshal(rep.Hardware, &hw); err != nil {
		t.Fatalf("hardware json: %v", err)
	}
	for _, k := range []string{"system", "software", "services", "startup_items", "connections", "source"} {
		if _, ok := hw[k]; !ok {
			t.Errorf("hardware missing %q", k)
		}
	}
}

func TestParseWinReportRejectsEmpty(t *testing.T) {
	if _, err := parseWinReport([]byte("  ")); err == nil {
		t.Error("expected error on empty output")
	}
	if _, err := parseWinReport([]byte(`{"os":"Windows"}`)); err == nil {
		t.Error("expected error when hostname missing")
	}
}

func TestClassify(t *testing.T) {
	cases := map[string]string{
		"0x80070005":        "access denied",
		"0x800706BA":        "unreachable",
		"x509: bad cert":    "cert",
	}
	for in, want := range cases {
		got := classify(errString(in), "").Error()
		if !contains(got, want) {
			t.Errorf("classify(%q)=%q, want contains %q", in, got, want)
		}
	}
}

type errString string

func (e errString) Error() string { return string(e) }
func contains(s, sub string) bool { return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
