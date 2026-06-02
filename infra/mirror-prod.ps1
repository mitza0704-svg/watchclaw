# mirror-prod.ps1 — pull the LIVE watchclaw DB from zmfbot and run a local,
# read-only mirror so UI changes can be verified on REAL data (screenshots)
# without touching production. This is the self-verification loop: no more
# "ship -> ask user to screenshot -> reject".
#
# Usage:  pwsh infra/mirror-prod.ps1            (then open http://127.0.0.1:18900/app)
#         $env:ZMF="zmf@100.68.90.68"           (override host if needed)
param(
  [string]$Zmf  = $(if ($env:ZMF) { $env:ZMF } else { "zmf@100.68.90.68" }),
  [int]   $Port = 18900
)
$ErrorActionPreference = "Stop"
$repo = Split-Path $PSScriptRoot -Parent
$db   = Join-Path $env:TEMP "wclive.db"
$exe  = Join-Path $repo "control-plane\watchclaw-cp.exe"

Write-Host "1/3 copy live DB out of the container on $Zmf ..."
ssh -o ConnectTimeout=12 -o StrictHostKeyChecking=no $Zmf "docker cp watchclaw-cp:/data/watchclaw.db /tmp/wclive.db && chmod 644 /tmp/wclive.db"

Write-Host "2/3 scp -> $db ..."
scp -o ConnectTimeout=12 -o StrictHostKeyChecking=no "${Zmf}:/tmp/wclive.db" $db
"{0:N1} MB pulled" -f ((Get-Item $db).Length/1MB) | Write-Host

Write-Host "3/3 build + run local read-only mirror on :$Port ..."
& "C:\Program Files\Go\bin\go.exe" build -o $exe (Join-Path $repo "control-plane")
Get-Process watchclaw-cp -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $exe } | Stop-Process -Force -ErrorAction SilentlyContinue
$env:WATCHCLAW_ADDR=":$Port"; $env:WATCHCLAW_DB=$db
$env:WATCHCLAW_MONITOR_INTERVAL="600s"; $env:WATCHCLAW_OFFLINE_AFTER="999m"  # read-only: no alert churn
$p = Start-Process $exe -PassThru -WindowStyle Hidden
Start-Sleep 2
Write-Host ("READY  http://127.0.0.1:{0}/app   (pid {1})  endpoints={2} nodes={3}" -f `
  $Port, $p.Id, (Invoke-RestMethod "http://127.0.0.1:$Port/v1/endpoints").Count, `
  (Invoke-RestMethod "http://127.0.0.1:$Port/v1/topology").nodes.Count)
