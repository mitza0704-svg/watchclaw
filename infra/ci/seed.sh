#!/usr/bin/env bash
# Seed a running control-plane with realistic demo data for CI screenshots.
set -euo pipefail
B="${1:-http://127.0.0.1:8787}"

curl -fsS -X POST "$B/v1/discovery" -H 'content-type: application/json' -d '{
 "subnet":"192.168.1.0/24","gateway":"192.168.1.1","scanned_at":"2026-01-01T00:00:00Z","host_count":254,"reporter":"ci",
 "devices":[
  {"ip":"192.168.1.1","hostname":"gateway","device_type":"gateway","nic_vendor":"TP-Link"},
  {"ip":"192.168.1.10","hostname":"WIN-SRV","device_type":"server","nic_vendor":"Dell"},
  {"ip":"192.168.1.11","hostname":"truenas","device_type":"nas","nic_vendor":"Synology"},
  {"ip":"192.168.1.20","hostname":"DESKTOP-CI","device_type":"workstation","nic_vendor":"Asus"},
  {"ip":"192.168.1.21","hostname":"DESKTOP-2","device_type":"workstation","nic_vendor":"Intel"},
  {"ip":"192.168.1.30","hostname":"HP-LaserJet","device_type":"printer","nic_vendor":"HP"},
  {"ip":"192.168.1.40","hostname":"ipcam","device_type":"camera","nic_vendor":"Hikvision"},
  {"ip":"192.168.1.50","hostname":"iPhone","device_type":"phone","nic_vendor":"Apple"},
  {"ip":"192.168.1.60","hostname":"shelly","device_type":"iot","nic_vendor":"Espressif"},
  {"ip":"192.168.1.61","hostname":"chromecast","device_type":"media","nic_vendor":"Google"}
 ]}' >/dev/null

curl -fsS -X POST "$B/v1/telemetry" -H 'content-type: application/json' -d '{
 "hostname":"DESKTOP-CI","os":"Windows","os_version":"26100","kernel_version":"10.0","cpu_cores":16,
 "cpu_usage_pct":23,"mem_total_mb":32768,"mem_used_mb":19000,"mem_usage_pct":58,"uptime_seconds":99000,
 "disks":[{"mount":"C:","total_gb":2000,"used_gb":1220,"usage_pct":61}],"collected_at":"2026-01-01T00:00:00Z",
 "hardware":{"system":{"model":"ROG Strix","manufacturer":"Asus","bios_serial":"CI12345","baseboard_serial":"BB99"},
  "available_updates":[{"name":"Git","id":"Git.Git","current":"2.53","available":"2.54"},{"name":"7-Zip","id":"7zip.7zip","current":"26.00","available":"26.01"}],
  "services":[{"name":"AnyDesk","display_name":"AnyDesk","state":"Running","start_mode":"Auto","path":"C:\\Program Files\\AnyDesk\\AnyDesk.exe --service"},{"name":"Spooler","state":"Running","start_mode":"Auto","path":"C:\\Windows\\System32\\spoolsv.exe"}],
  "startup_items":[{"name":"SecurityHealth","location":"HKLM\\Run","command":"%windir%\\system32\\SecurityHealthSystray.exe"}],
  "software":[{"name":"Chrome","version":"120"},{"name":"VS Code","version":"1.9"}],
  "usb_devices":[{"name":"USB Hub","vid":"046d","pid":"c52b"}],
  "connections":[
   {"local":"192.168.1.20:5001","remote":"192.168.1.10:445","state":"Established","pid":4,"process":"System"},
   {"local":"192.168.1.20:5002","remote":"192.168.1.11:445","state":"Established","pid":4,"process":"System"},
   {"local":"192.168.1.20:5003","remote":"192.168.1.10:3389","state":"Established","pid":900,"process":"mstsc.exe"},
   {"local":"192.168.1.20:5004","remote":"160.79.104.10:443","state":"Established","pid":800,"process":"chrome.exe"},
   {"local":"192.168.1.20:5005","remote":"35.157.63.229:443","state":"Established","pid":810,"process":"AteraAgent.exe"},
   {"local":"0.0.0.0:445","remote":"0.0.0.0:0","state":"Listen","pid":4,"process":"System"},
   {"local":"0.0.0.0:3389","remote":"0.0.0.0:0","state":"Listen","pid":7,"process":"svchost.exe"}
  ]}}' >/dev/null

echo "seeded $B"
