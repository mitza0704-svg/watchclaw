// Package topology fuses a network scan into a graph for the dashboard.
//
// F0 builds an L3 star: Internet -> gateway -> devices. The gateway is the real
// default gateway reported by the collector (falls back to the .1 heuristic only
// if the collector couldn't read the routing table). Device labels prefer the
// real hostname; the MAC OUI is kept only as "NIC vendor" metadata, never as the
// device identity (an Asus PC can carry a TP-Link NIC). True port-level L2
// topology arrives with the SNMP/LLDP collector.
package topology

import (
	"net"
	"sort"
	"strings"

	"github.com/fullstackit/watchclaw/control-plane/internal/model"
)

const (
	nodeInternet = "internet"
	nodeGateway  = "gateway"
)

func Build(scan *model.NetworkScan) model.TopologyGraph {
	g := model.TopologyGraph{Subnet: scan.Subnet}
	if scan == nil || len(scan.Devices) == 0 {
		return g
	}

	gatewayIP := scan.Gateway
	if gatewayIP == "" {
		gatewayIP = inferGateway(scan.Devices) // fallback if routing table unavailable
	}

	g.Nodes = append(g.Nodes,
		model.TopoNode{ID: nodeInternet, Label: "Internet", Type: "internet"},
		model.TopoNode{ID: nodeGateway, Label: "Gateway · " + gatewayIP, Type: "gateway", IP: gatewayIP},
	)
	g.Edges = append(g.Edges, model.TopoEdge{From: nodeInternet, To: nodeGateway, Type: "wan"})

	for _, d := range scan.Devices {
		if d.IP == gatewayIP {
			for i := range g.Nodes {
				if g.Nodes[i].ID == nodeGateway {
					g.Nodes[i].MAC = d.MAC
					g.Nodes[i].NicVendor = d.NicVendor
					g.Nodes[i].Hostname = d.Hostname
					g.Nodes[i].OpenPorts = d.OpenPorts
					if d.Hostname != "" {
						g.Nodes[i].Label = d.Hostname + " · gateway"
					} else if d.NicVendor != "" {
						g.Nodes[i].Label = gatewayIP + " · gateway (" + d.NicVendor + " NIC)"
					}
				}
			}
			continue
		}
		id := "dev-" + d.IP
		g.Nodes = append(g.Nodes, model.TopoNode{
			ID:        id,
			Label:     deviceLabel(d),
			Type:      deviceType(d),
			IP:        d.IP,
			MAC:       d.MAC,
			Hostname:  d.Hostname,
			NicVendor: d.NicVendor,
			OpenPorts: d.OpenPorts,
		})
		g.Edges = append(g.Edges, model.TopoEdge{From: nodeGateway, To: id, Type: "l3"})
	}
	return g
}

// deviceLabel prefers the real hostname; otherwise the IP. The NIC vendor is
// shown only as a parenthetical hint, never as the identity.
func deviceLabel(d model.NetworkDevice) string {
	if d.Hostname != "" {
		return d.Hostname
	}
	if d.NicVendor != "" {
		return d.IP + " (" + d.NicVendor + " NIC)"
	}
	return d.IP
}

// deviceType uses the agent's fingerprint when present, falling back to a port
// heuristic for older reports.
func deviceType(d model.NetworkDevice) string {
	if d.DeviceType != "" {
		return d.DeviceType
	}
	for _, p := range d.OpenPorts {
		if p == 9100 {
			return "printer"
		}
		if p == 445 || p == 3389 || p == 135 {
			return "workstation"
		}
	}
	return "device"
}

func inferGateway(devices []model.NetworkDevice) string {
	ips := make([]string, 0, len(devices))
	for _, d := range devices {
		ips = append(ips, d.IP)
	}
	for _, ip := range ips {
		if strings.HasSuffix(ip, ".1") {
			return ip
		}
	}
	sort.Slice(ips, func(i, j int) bool { return ipLess(ips[i], ips[j]) })
	if len(ips) > 0 {
		return ips[0]
	}
	return ""
}

func ipLess(a, b string) bool {
	ia, ib := net.ParseIP(a).To4(), net.ParseIP(b).To4()
	if ia == nil || ib == nil {
		return a < b
	}
	for i := 0; i < 4; i++ {
		if ia[i] != ib[i] {
			return ia[i] < ib[i]
		}
	}
	return false
}
