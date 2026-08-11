// BGP IPv4 test for ngn_arrcus topology using IXIA-C-One via gosnappi.
//
// Topology connections (ixia-c port → DUT):
//
//	eth1  <-> ec-gw1:eth4            IXIA 10.239.4.237/31,  BGP ASN 7018
//	eth2  <-> arcos-leaf-a-side:swp2 IXIA 206.111.1.3/31,   BGP ASN 90000 (4-byte), MD5 "equinix"
//	eth3  <-> arcos-leaf-z-side:swp2 IXIA 192.168.70.2/24,  BGP ASN 16302,          MD5 "equinix"
//
// Run:
//
//	go test -v -run TestBgpIPv4 -otg-host https://<ixia-c-mgmt-ip>:<nodeport>
package main

import (
	"flag"
	"fmt"
	"log"
	"testing"
	"time"

	gosnappi "github.com/open-traffic-generator/snappi/gosnappi"
)

var otgHost = flag.String("otg-host", "https://localhost:30412", "OTG API endpoint of IXIA-C-One")

// OTG port names
const (
	pEcGw  = "p_ecgw"
	pSideA = "p_side_a"
	pSideZ = "p_side_z"
)

// IXIA-C-One Linux interface names (inside the container)
const (
	locEcGw  = "eth1"
	locSideA = "eth2"
	locSideZ = "eth3"
)

// ec-gw port (ixia-c:eth1 <-> ec-gw1:eth4)
const (
	ecgwIxiaIP   = "10.239.4.237"
	ecgwGW       = "10.239.4.236"
	ecgwPrefix   = 31
	ecgwIxiaMAC  = "00:00:01:01:01:01"
	ecgwASN      = 7018
	ecgwRouteNet = "1.1.1.0"
	ecgwRouteLen = 24
	ecgwRouteQty = 1
)

// arrcus-side-a port (ixia-c:eth2 <-> arcos-leaf-a-side:swp2)
const (
	sideAIxiaIP   = "206.111.1.3"
	sideAGW       = "206.111.1.2"
	sideAPrefix   = 31
	sideAIxiaMAC  = "00:00:02:02:02:02"
	sideAASN      = 90000 // 4-byte ASN
	sideARouteNet = "8.8.8.0"
	sideARouteLen = 24
	sideARouteQty = 1
	sideAMD5Key   = "equinix"
)

// arrcus-side-z port (ixia-c:eth3 <-> arcos-leaf-z-side:swp2)
const (
	sideZIxiaIP   = "192.168.70.2"
	sideZGW       = "192.168.70.1"
	sideZPrefix   = 24
	sideZIxiaMAC  = "00:00:03:03:03:03"
	sideZASN      = 16302
	sideZRouteNet = "8.8.8.0"
	sideZRouteLen = 24
	sideZRouteQty = 1
	sideZMD5Key   = "equinix"
)

func TestBgpIPv4(t *testing.T) {
	flag.Parse()

	api := gosnappi.NewApi()
	api.NewHttpTransport().SetLocation(*otgHost).SetVerify(false)

	config := newBgpConfig()

	log.Println("Pushing OTG config...")
	if _, err := api.SetConfig(config); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	log.Println("Starting protocols...")
	cs := gosnappi.NewControlState()
	cs.Protocol().All().SetState(gosnappi.StateProtocolAllState.START)
	if _, err := api.SetControlState(cs); err != nil {
		t.Fatalf("start protocols: %v", err)
	}

	log.Println("Waiting 30s for BGP sessions to establish...")
	time.Sleep(30 * time.Second)

	if err := verifyBgpSessions(api); err != nil {
		t.Fatalf("BGP verification: %v", err)
	}

	log.Println("Starting traffic flows...")
	ts := gosnappi.NewControlState()
	ts.Traffic().FlowTransmit().SetState(gosnappi.StateTrafficFlowTransmitState.START)
	if _, err := api.SetControlState(ts); err != nil {
		t.Fatalf("start traffic: %v", err)
	}

	time.Sleep(10 * time.Second)

	log.Println("Stopping traffic flows...")
	ts2 := gosnappi.NewControlState()
	ts2.Traffic().FlowTransmit().SetState(gosnappi.StateTrafficFlowTransmitState.STOP)
	if _, err := api.SetControlState(ts2); err != nil {
		t.Fatalf("stop traffic: %v", err)
	}

	if err := verifyFlowMetrics(api); err != nil {
		t.Fatalf("flow metrics: %v", err)
	}
}

// newBgpConfig builds the full OTG configuration for the ngn_arrcus topology.
func newBgpConfig() gosnappi.Config {
	config := gosnappi.NewConfig()

	// ── Ports ─────────────────────────────────────────────────────────────
	portEcGw := config.Ports().Add().SetName(pEcGw).SetLocation(locEcGw)
	portSideA := config.Ports().Add().SetName(pSideA).SetLocation(locSideA)
	portSideZ := config.Ports().Add().SetName(pSideZ).SetLocation(locSideZ)

	// ── Device: ec-gw ─────────────────────────────────────────────────────
	devEcGw := config.Devices().Add().SetName("dev_ecgw")

	ethEcGw := devEcGw.Ethernets().Add().SetName("eth_ecgw").SetMac(ecgwIxiaMAC)
	ethEcGw.Connection().SetPortName(portEcGw.Name())

	ipEcGw := ethEcGw.Ipv4Addresses().Add().
		SetName("ip_ecgw").
		SetAddress(ecgwIxiaIP).
		SetGateway(ecgwGW).
		SetPrefix(ecgwPrefix)

	bgpEcGw := devEcGw.Bgp().SetRouterId(ecgwIxiaIP)
	bgpIfEcGw := bgpEcGw.Ipv4Interfaces().Add().SetIpv4Name(ipEcGw.Name())
	bgpPeerEcGw := bgpIfEcGw.Peers().Add().
		SetName("bgp_ecgw").
		SetPeerAddress(ecgwGW).
		SetAsNumber(ecgwASN).
		SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	bgpPeerEcGw.V4Routes().Add().
		SetName("routes_ecgw").
		Addresses().Add().
		SetAddress(ecgwRouteNet).
		SetPrefix(ecgwRouteLen).
		SetCount(ecgwRouteQty)

	// ── Device: arrcus-side-a ─────────────────────────────────────────────
	devSideA := config.Devices().Add().SetName("dev_side_a")

	ethSideA := devSideA.Ethernets().Add().SetName("eth_side_a").SetMac(sideAIxiaMAC)
	ethSideA.Connection().SetPortName(portSideA.Name())

	ipSideA := ethSideA.Ipv4Addresses().Add().
		SetName("ip_side_a").
		SetAddress(sideAIxiaIP).
		SetGateway(sideAGW).
		SetPrefix(sideAPrefix)

	bgpSideA := devSideA.Bgp().SetRouterId(sideAIxiaIP)
	bgpIfSideA := bgpSideA.Ipv4Interfaces().Add().SetIpv4Name(ipSideA.Name())
	bgpPeerSideA := bgpIfSideA.Peers().Add().
		SetName("bgp_side_a").
		SetPeerAddress(sideAGW).
		SetAsNumber(sideAASN).
		SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	bgpPeerSideA.Advanced().SetMd5Key(sideAMD5Key)

	bgpPeerSideA.V4Routes().Add().
		SetName("routes_side_a").
		Addresses().Add().
		SetAddress(sideARouteNet).
		SetPrefix(sideARouteLen).
		SetCount(sideARouteQty)

	// ── Device: arrcus-side-z ─────────────────────────────────────────────
	devSideZ := config.Devices().Add().SetName("dev_side_z")

	ethSideZ := devSideZ.Ethernets().Add().SetName("eth_side_z").SetMac(sideZIxiaMAC)
	ethSideZ.Connection().SetPortName(portSideZ.Name())

	ipSideZ := ethSideZ.Ipv4Addresses().Add().
		SetName("ip_side_z").
		SetAddress(sideZIxiaIP).
		SetGateway(sideZGW).
		SetPrefix(sideZPrefix)

	bgpSideZ := devSideZ.Bgp().SetRouterId(sideZIxiaIP)
	bgpIfSideZ := bgpSideZ.Ipv4Interfaces().Add().SetIpv4Name(ipSideZ.Name())
	bgpPeerSideZ := bgpIfSideZ.Peers().Add().
		SetName("bgp_side_z").
		SetPeerAddress(sideZGW).
		SetAsNumber(sideZASN).
		SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	bgpPeerSideZ.Advanced().SetMd5Key(sideZMD5Key)

	bgpPeerSideZ.V4Routes().Add().
		SetName("routes_side_z").
		Addresses().Add().
		SetAddress(sideZRouteNet).
		SetPrefix(sideZRouteLen).
		SetCount(sideZRouteQty)

	// ── Flows ──────────────────────────────────────────────────────────────
	// ec-gw ↔ arrcus-side-a
	addFlow(config, "flow_ecgw_to_side_a",
		ecgwIxiaIP, sideARouteNet,
		"routes_ecgw", "routes_side_a")

	addFlow(config, "flow_side_a_to_ecgw",
		sideAIxiaIP, ecgwRouteNet,
		"routes_side_a", "routes_ecgw")

	// ec-gw ↔ arrcus-side-z
	addFlow(config, "flow_ecgw_to_side_z",
		ecgwIxiaIP, sideZRouteNet,
		"routes_ecgw", "routes_side_z")

	addFlow(config, "flow_side_z_to_ecgw",
		sideZIxiaIP, ecgwRouteNet,
		"routes_side_z", "routes_ecgw")

	// arrcus-side-a ↔ arrcus-side-z
	addFlow(config, "flow_side_a_to_side_z",
		sideAIxiaIP, sideZRouteNet,
		"routes_side_a", "routes_side_z")

	addFlow(config, "flow_side_z_to_side_a",
		sideZIxiaIP, sideARouteNet,
		"routes_side_z", "routes_side_a")

	return config
}

// addFlow adds a fixed-rate IPv4/UDP flow between two BGP route sets.
func addFlow(config gosnappi.Config, name, srcIP, dstIP, txRoute, rxRoute string) {
	flow := config.Flows().Add().SetName(name)

	flow.TxRx().Device().
		SetTxNames([]string{txRoute}).
		SetRxNames([]string{rxRoute})

	flow.Size().SetFixed(512)
	flow.Rate().SetPps(1000)
	flow.Duration().FixedPackets().SetPackets(10000)
	flow.Metrics().SetEnable(true)

	ip := flow.Packet().Add().Ipv4()
	ip.Src().SetValue(srcIP)
	ip.Dst().SetValue(dstIP)

	udp := flow.Packet().Add().Udp()
	udp.SrcPort().SetValue(5000)
	udp.DstPort().SetValue(5001)
}

// verifyBgpSessions checks that all three BGP peers reach UP state.
func verifyBgpSessions(api gosnappi.Api) error {
	req := gosnappi.NewMetricsRequest()
	req.Bgpv4()

	resp, err := api.GetMetrics(req)
	if err != nil {
		return fmt.Errorf("GetMetrics bgpv4: %w", err)
	}

	up := map[string]bool{
		"bgp_ecgw":   false,
		"bgp_side_a": false,
		"bgp_side_z": false,
	}

	for _, m := range resp.Bgpv4Metrics().Items() {
		state := m.SessionState()
		log.Printf("BGP peer %-20s  state: %s", m.Name(), state)
		if state == gosnappi.Bgpv4MetricSessionState.UP {
			up[m.Name()] = true
		}
	}

	for peer, established := range up {
		if !established {
			return fmt.Errorf("BGP peer %q not UP", peer)
		}
	}

	log.Println("All BGP sessions UP.")
	return nil
}

// verifyFlowMetrics asserts zero packet loss across all flows.
func verifyFlowMetrics(api gosnappi.Api) error {
	req := gosnappi.NewMetricsRequest()
	req.Flow()

	resp, err := api.GetMetrics(req)
	if err != nil {
		return fmt.Errorf("GetMetrics flow: %w", err)
	}

	var failed bool
	for _, m := range resp.FlowMetrics().Items() {
		tx := m.FramesTx()
		rx := m.FramesRx()
		loss := tx - rx
		log.Printf("Flow %-40s  tx=%-8d rx=%-8d loss=%d", m.Name(), tx, rx, loss)
		if loss > 0 {
			log.Printf("  FAIL: packet loss on flow %s", m.Name())
			failed = true
		}
	}

	if failed {
		return fmt.Errorf("one or more flows had packet loss")
	}

	log.Println("All flows passed with zero loss.")
	return nil
}
