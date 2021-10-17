package netlink

import (
	"context"
	"fmt"
	"net"

	remoteNetlink "github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"k8s.io/klog/v2"
)

var _ context.Context

type bridgeType string
type vethType string
type DockerType string
type CrioType int

// ToDo: make this variable later
const (
	DefaultExternalBridgeName = "externalBridge"
	DefaultInternalBridgeName = "internalBridge"

	TYPEVETH   = "veth"
	TYPEBRIDGE = "bridge"

	DefaultInternalContainerInterface = "ethint"
	DefaultExternalContainerInterface = "ethext"
)

type snapshot struct {
	intIfname string
	extIfname string

	newIntIfname []string
	newExtIfname []string

	intIPAddrs []net.Addr
	extIPAddrs []net.Addr

	defaultGW net.IP
}

var originSnapshot *snapshot

func Initialize(cfg *Config) error {
	var rootNetlinkHandle *remoteNetlink.Handle
	var err error

	originSnapshot = &snapshot{
		newIntIfname: make([]string, 0),
		newExtIfname: make([]string, 0),
		intIPAddrs:   make([]net.Addr, 0),
		extIPAddrs:   make([]net.Addr, 0),
	}

	if rootNetlinkHandle, err = GetRootNetlinkHandle(); err != nil {
		klog.ErrorS(err, "Initializing failed while getting rootNetlinkHandle")
		return err
	}

	if _, err := setExternalBridge(rootNetlinkHandle, cfg); err != nil {
		klog.ErrorS(err, "Initializing failed while setting ExternalBridge")
		return err
	}

	if _, err := setInternalBridge(rootNetlinkHandle, cfg); err != nil {
		klog.ErrorS(err, "Initializing failed while setting InternalBridge")
		return err
	}

	originSnapshot.defaultGW = getDefaultGW()

	if err := initInternalInterface(rootNetlinkHandle, cfg); err != nil {
		klog.ErrorS(err, "Initializing failed while setting InternalInterface")
	}

	if err := initExternalInterface(rootNetlinkHandle, cfg); err != nil {
		klog.ErrorS(err, "Initializing failed while setting InternalInterface")
	}

	klog.InfoS("Origin Addr", "intAddr", originSnapshot.intIPAddrs, "extAddr", originSnapshot.extIPAddrs)
	return nil
}

func Clear(cfg *Config) error {
	var rootNetlinkHandle *remoteNetlink.Handle
	var err error

	if rootNetlinkHandle, err = GetRootNetlinkHandle(); err != nil {
		klog.ErrorS(err, "Initializing failed while getting rootNetlinkHandle")
		return err
	}

	if err := clearExternalBridge(rootNetlinkHandle, cfg); err != nil {
		klog.ErrorS(err, "Clearing failed while deleting ExternalBridge")
		return err
	}

	if err := clearInternalBridge(rootNetlinkHandle, cfg); err != nil {
		klog.ErrorS(err, "Clearing failed while deleting InternalBridge")
		return err
	}

	klog.InfoS("Origin Addr", "addr", originSnapshot.intIPAddrs)
	// restore ip to origin
	var originIntInterface remoteNetlink.Link
	var originExtInterface remoteNetlink.Link

	if link, err := rootNetlinkHandle.LinkByName(originSnapshot.intIfname); err != nil {
		klog.ErrorS(err, "LinkByName is failed", "interfaceName", originSnapshot.intIfname)
		return err
	} else {
		originIntInterface = link
	}

	for _, addr := range originSnapshot.intIPAddrs {
		if a, err := remoteNetlink.ParseAddr(addr.String()); err != nil {
			klog.ErrorS(err, "ParseAddr is failed", "addr", addr.String())
			return err
		} else {
			if err := rootNetlinkHandle.AddrAdd(originIntInterface, a); err != nil {
				klog.ErrorS(err, "AddrAdd is failed", "originIntInterface", originIntInterface.Attrs().Name, "addr", a.IPNet.String())
				return err
			} else {
				klog.InfoS("AddrAdd is done", "originIntInterface", originIntInterface.Attrs().Name, "addr", a.IPNet.String())
			}
		}
	}

	if link, err := rootNetlinkHandle.LinkByName(originSnapshot.extIfname); err != nil {
		klog.ErrorS(err, "LinkByName is failed", "interfaceName", originSnapshot.extIfname)
		return err
	} else {
		originExtInterface = link
	}

	for _, addr := range originSnapshot.extIPAddrs {
		if a, err := remoteNetlink.ParseAddr(addr.String()); err != nil {
			klog.ErrorS(err, "ParseAddr is failed", "addr", addr.String())
			return err
		} else {
			if err := rootNetlinkHandle.AddrAdd(originExtInterface, a); err != nil {
				klog.ErrorS(err, "AddrAdd is failed", "originExtInterface", originExtInterface.Attrs().Name, "addr", a.IPNet.String())
				return err
			} else {
				klog.InfoS("AddrAdd is done", "originExtInterface", originExtInterface.Attrs().Name, "addr", a.IPNet.String())
			}
		}
	}

	if err := setLinkUp(rootNetlinkHandle, originIntInterface); err != nil {
		klog.ErrorS(err, "setLinkUp is failed", "interfaceName", originIntInterface.Attrs().Name)
		return err
	}

	if err := setLinkUp(rootNetlinkHandle, originExtInterface); err != nil {
		klog.ErrorS(err, "setLinkUp is failed", "interfaceName", originExtInterface.Attrs().Name)
		return err
	}

	for _, newIntInterface := range originSnapshot.newIntIfname {
		if err := clearVethInterface(rootNetlinkHandle, newIntInterface); err != nil {
			klog.ErrorS(err, "Clearing failed while deleting Internal Interfae")
			return err
		}
	}

	for _, newExtInterface := range originSnapshot.newExtIfname {
		if err := clearVethInterface(rootNetlinkHandle, newExtInterface); err != nil {
			klog.ErrorS(err, "Clearing failed while deleting External Interface")
			return err
		}
	}

	if err := setDefaultGW(originSnapshot.defaultGW); err != nil {
		klog.ErrorS(err, "setDefaultGW failed", "gw", originSnapshot.defaultGW)
		return err
	}

	return nil
}

//ToDo: implement external side
func initInternalInterface(rootNetlinkHandle *remoteNetlink.Handle, cfg *Config) error {
	var network *net.IPNet

	if _, net, err := net.ParseCIDR(cfg.InternalIPCIDR); err != nil {
		klog.ErrorS(err, "Parsing internalIPCIDR is failed", "internalIPCIDR", cfg.InternalIPCIDR)
		return err
	} else {
		network = net
	}

	if ifaces, err := net.Interfaces(); err != nil {
		klog.ErrorS(err, "Listing Interfaces failed")
		return err
	} else {
		for _, i := range ifaces {
			if addrs, err := i.Addrs(); err != nil {
				klog.ErrorS(err, "Retrieve Addr object is failed", "interfaceName", i.Name)
				return err
			} else {
				for _, addr := range addrs {
					if c, _, err := net.ParseCIDR(addr.String()); err != nil {
						klog.ErrorS(err, "ParseCIDR is failed", "addr", addr.String())
					} else {
						if network.Contains(c) {
							originSnapshot.intIPAddrs = append(originSnapshot.intIPAddrs, addr)
							originSnapshot.intIfname = i.Name
						}
					}
				}
			}
		}
	}

	var originLink remoteNetlink.Link
	var bridgeLink remoteNetlink.Link
	var newVethLink remoteNetlink.Link
	var newVethPeerLink remoteNetlink.Link

	if link, err := rootNetlinkHandle.LinkByName(originSnapshot.intIfname); err != nil {
		klog.ErrorS(err, "LinkByName is failed", "interfaceName", originSnapshot.intIfname)
		return err
	} else {
		originLink = link
	}
	if link, err := rootNetlinkHandle.LinkByName(cfg.InternalBridgeName); err != nil {
		klog.ErrorS(err, "LinkByName is failed", "interfaceName", cfg.InternalBridgeName)
		return err
	} else {
		bridgeLink = link
	}

	veth := &remoteNetlink.Veth{
		LinkAttrs: remoteNetlink.LinkAttrs{
			Name: cfg.InternalInterfaceName + "0",
		},
		PeerName: cfg.InternalInterfaceName + "1",
		// PeerHardwareAddr: originLink.Attrs().HardwareAddr,
	}

	if link, err := setLinkbyLink(rootNetlinkHandle, veth); err != nil {
		klog.ErrorS(err, "setLink failed", "interfaceName", veth.Attrs().Name)
		return err
	} else {
		newVethLink = link
	}
	if link, err := rootNetlinkHandle.LinkByName(veth.PeerName); err != nil {
		klog.ErrorS(err, "LinkByName is failed", "interfaceName", veth.PeerName)
		return err
	} else {
		newVethPeerLink = link
	}

	originSnapshot.newIntIfname = append(originSnapshot.newIntIfname, newVethLink.Attrs().Name)

	if err := attachInterface2Bridge(rootNetlinkHandle, originLink, bridgeLink); err != nil {
		klog.ErrorS(err, "attachInterface2Bridge is failed", "interfaceName", originLink.Attrs().Name, "bridgeName", bridgeLink.Attrs().Name)
		return err
	}

	if err := attachInterface2Bridge(rootNetlinkHandle, newVethLink, bridgeLink); err != nil {
		klog.ErrorS(err, "attachInterface2Bridge is failed", "interfaceName", newVethLink.Attrs().Name, "bridgeName", bridgeLink.Attrs().Name)
		return err
	}

	for _, addr := range originSnapshot.intIPAddrs {
		if a, err := remoteNetlink.ParseAddr(addr.String()); err != nil {
			klog.ErrorS(err, "ParseAddr is failed", "addr", addr.String())
			return err
		} else {
			if err := rootNetlinkHandle.AddrAdd(newVethPeerLink, a); err != nil {
				klog.ErrorS(err, "AddrAdd is failed", "Link Name", newVethPeerLink.Attrs().Name, "addr", a.IPNet.String())
				return err
			} else {
				klog.InfoS("AddrAdd is done", "Link Name", newVethPeerLink.Attrs().Name, "addr", a.IPNet.String())
			}
		}
	}

	for _, addr := range originSnapshot.intIPAddrs {
		if a, err := remoteNetlink.ParseAddr(addr.String()); err != nil {
			klog.ErrorS(err, "ParseAddr is failed", "addr", addr.String())
			return err
		} else {
			if err := rootNetlinkHandle.AddrDel(originLink, a); err != nil {
				klog.ErrorS(err, "AddrDel is failed", "Link Name", originLink.Attrs().Name, "addr", a.IPNet.String())
				return err
			} else {
				klog.InfoS("AddrDel is done", "Link Name", originLink.Attrs().Name, "addr", a.IPNet.String())
			}
		}
	}

	if err := setLinkUp(rootNetlinkHandle, newVethLink); err != nil {
		klog.ErrorS(err, "setLinkUp is failed", "interfaceName", newVethLink.Attrs().Name)
		return err
	}
	if err := setLinkUp(rootNetlinkHandle, newVethPeerLink); err != nil {
		klog.ErrorS(err, "setLinkUp is failed", "interfaceName", newVethPeerLink.Attrs().Name)
		return err
	}
	if err := setLinkUp(rootNetlinkHandle, originLink); err != nil {
		klog.ErrorS(err, "setLinkUp is failed", "interfaceName", originLink.Attrs().Name)
		return err
	}
	if err := setLinkUp(rootNetlinkHandle, bridgeLink); err != nil {
		klog.ErrorS(err, "setLinkUp is failed", "interfaceName", originLink.Attrs().Name)
		return err
	}

	return nil
}

func initExternalInterface(rootNetlinkHandle *remoteNetlink.Handle, cfg *Config) error {
	var network *net.IPNet

	if _, net, err := net.ParseCIDR(cfg.ExternalIPCIDR); err != nil {
		klog.ErrorS(err, "Parsing externalIPCIDR is failed", "externalIPCIDR", cfg.ExternalIPCIDR)
		return err
	} else {
		network = net
	}

	if ifaces, err := net.Interfaces(); err != nil {
		klog.ErrorS(err, "Listing Interfaces failed")
		return err
	} else {
		for _, i := range ifaces {
			if addrs, err := i.Addrs(); err != nil {
				klog.ErrorS(err, "Retrieve Addr object is failed", "interfaceName", i.Name)
				return err
			} else {
				for _, addr := range addrs {
					if c, _, err := net.ParseCIDR(addr.String()); err != nil {
						klog.ErrorS(err, "ParseCIDR is failed", "addr", addr.String())
					} else {
						if network.Contains(c) {
							originSnapshot.extIPAddrs = append(originSnapshot.extIPAddrs, addr)
							originSnapshot.extIfname = i.Name
						}
					}
				}
			}
		}
	}

	var originLink remoteNetlink.Link
	var bridgeLink remoteNetlink.Link
	var newVethLink remoteNetlink.Link
	var newVethPeerLink remoteNetlink.Link

	if link, err := rootNetlinkHandle.LinkByName(originSnapshot.extIfname); err != nil {
		klog.ErrorS(err, "LinkByName is failed", "interfaceName", originSnapshot.extIfname)
		return err
	} else {
		originLink = link
	}
	if link, err := rootNetlinkHandle.LinkByName(cfg.ExternalBridgeName); err != nil {
		klog.ErrorS(err, "LinkByName is failed", "interfaceName", cfg.ExternalBridgeName)
		return err
	} else {
		bridgeLink = link
	}

	veth := &remoteNetlink.Veth{
		LinkAttrs: remoteNetlink.LinkAttrs{
			Name: cfg.ExternalInterfaceName + "0",
		},
		PeerName: cfg.ExternalInterfaceName + "1",
		// PeerHardwareAddr: originLink.Attrs().HardwareAddr,
	}

	if link, err := setLinkbyLink(rootNetlinkHandle, veth); err != nil {
		klog.ErrorS(err, "setLink failed", "interfaceName", veth.Attrs().Name)
		return err
	} else {
		newVethLink = link
	}
	if link, err := rootNetlinkHandle.LinkByName(veth.PeerName); err != nil {
		klog.ErrorS(err, "LinkByName is failed", "interfaceName", veth.PeerName)
		return err
	} else {
		newVethPeerLink = link
	}

	if newVethLink == nil {
		klog.Error("link nil")
	}
	if newVethLink.Attrs() == nil {
		klog.Error("link attrs nil")
	}
	if newVethLink.Attrs().Name == "" {
		klog.Error("Name nil")
	}

	originSnapshot.newExtIfname = append(originSnapshot.newExtIfname, newVethLink.Attrs().Name)

	if err := attachInterface2Bridge(rootNetlinkHandle, originLink, bridgeLink); err != nil {
		klog.ErrorS(err, "attachInterface2Bridge is failed", "interfaceName", originLink.Attrs().Name, "bridgeName", bridgeLink.Attrs().Name)
		return err
	}

	if err := attachInterface2Bridge(rootNetlinkHandle, newVethLink, bridgeLink); err != nil {
		klog.ErrorS(err, "attachInterface2Bridge is failed", "interfaceName", newVethLink.Attrs().Name, "bridgeName", bridgeLink.Attrs().Name)
		return err
	}

	for _, addr := range originSnapshot.extIPAddrs {
		if a, err := remoteNetlink.ParseAddr(addr.String()); err != nil {
			klog.ErrorS(err, "ParseAddr is failed", "addr", addr.String())
			return err
		} else {
			if err := rootNetlinkHandle.AddrAdd(newVethPeerLink, a); err != nil {
				klog.ErrorS(err, "AddrAdd is failed", "Link Name", newVethPeerLink.Attrs().Name, "addr", a.IPNet.String())
				return err
			} else {
				klog.InfoS("AddrAdd is done", "Link Name", newVethPeerLink.Attrs().Name, "addr", a.IPNet.String())
			}
		}
	}

	for _, addr := range originSnapshot.extIPAddrs {
		if a, err := remoteNetlink.ParseAddr(addr.String()); err != nil {
			klog.ErrorS(err, "ParseAddr is failed", "addr", addr.String())
			return err
		} else {
			if err := rootNetlinkHandle.AddrDel(originLink, a); err != nil {
				klog.ErrorS(err, "AddrDel is failed", "Link Name", originLink.Attrs().Name, "addr", a.IPNet.String())
				return err
			} else {
				klog.InfoS("AddrDel is done", "Link Name", originLink.Attrs().Name, "addr", a.IPNet.String())
			}
		}
	}

	if err := setLinkUp(rootNetlinkHandle, newVethLink); err != nil {
		klog.ErrorS(err, "setLinkUp is failed", "interfaceName", newVethLink.Attrs().Name)
		return err
	}
	if err := setLinkUp(rootNetlinkHandle, newVethPeerLink); err != nil {
		klog.ErrorS(err, "setLinkUp is failed", "interfaceName", newVethPeerLink.Attrs().Name)
		return err
	}
	if err := setLinkUp(rootNetlinkHandle, originLink); err != nil {
		klog.ErrorS(err, "setLinkUp is failed", "interfaceName", originLink.Attrs().Name)
		return err
	}
	if err := setLinkUp(rootNetlinkHandle, bridgeLink); err != nil {
		klog.ErrorS(err, "setLinkUp is failed", "interfaceName", originLink.Attrs().Name)
		return err
	}

	if err := setDefaultGW(originSnapshot.defaultGW); err != nil {
		klog.ErrorS(err, "setDefaultGW failed", "gw", originSnapshot.defaultGW)
		return err
	} else {
		klog.InfoS("setDefaultGW done", "gw", originSnapshot.defaultGW)
	}

	// dst := &net.IPNet{
	// 	IP:   net.IPv4(0, 0, 0, 0),
	// 	Mask: net.CIDRMask(0, 32),
	// }
	// gw := net.IPv4(192, 168, 7, 1)
	// route := remoteNetlink.Route{Dst: dst, Gw: gw, LinkIndex: newVethPeerLink.Attrs().Index}
	// if err := rootNetlinkHandle.RouteAdd(&route); err != nil {
	// 	klog.ErrorS(err, "Default RouteAdd failed", "route", route.String())
	// 	return err
	// }

	return nil
}
func DD() {
	a, _ := remoteNetlink.RouteList(nil, 2)
	for _, v := range a {
		fmt.Println(v)
	}
	// fmt.Println(netlink.RouteList(nil, 2))
	fmt.Println(getDefaultGW())
}

func getDefaultGW() net.IP {
	n, _ := remoteNetlink.RouteList(nil, 2)
	for _, v := range n {
		if v.Dst == nil {
			return v.Gw
		}
	}
	return nil
}

func setDefaultGW(gw net.IP) error {
	// dst := &net.IPNet{
	// 	IP:   net.IPv4(0, 0, 0, 0),
	// 	Mask: net.CIDRMask(0, 32),
	// }
	return remoteNetlink.RouteAdd(&remoteNetlink.Route{
		Dst: nil,
		Gw:  gw,
	})
}

func attachInterface2Bridge(rootNetlinkHandle *remoteNetlink.Handle, networkInterface remoteNetlink.Link, bridge remoteNetlink.Link) error {
	if err := rootNetlinkHandle.LinkSetMaster(networkInterface, bridge); err != nil {
		klog.ErrorS(err, "Attaching Interface to Bridge failed", "Interface", networkInterface.Attrs().Name, "Bridge", bridge.Attrs().Name)
		return err
	}

	if err := rootNetlinkHandle.LinkSetUp(networkInterface); err != nil {
		klog.ErrorS(err, "Setting up interface failed", "Interface", networkInterface.Attrs().Name)
		return err
	}
	if err := rootNetlinkHandle.LinkSetUp(bridge); err != nil {
		klog.ErrorS(err, "Setting up bridge failed", "Interface", bridge.Attrs().Name)
		return err
	}
	return nil
}

func clearExternalBridge(rootNetlinkHandle *remoteNetlink.Handle, cfg *Config) error {
	if cfg.ExternalBridgeName == "" {
		return clearBridge(rootNetlinkHandle, DefaultExternalBridgeName)
	}
	return clearBridge(rootNetlinkHandle, cfg.ExternalBridgeName)
}

func clearInternalBridge(rootNetlinkHandle *remoteNetlink.Handle, cfg *Config) error {
	if cfg.InternalBridgeName == "" {
		return clearBridge(rootNetlinkHandle, DefaultInternalBridgeName)
	}
	return clearBridge(rootNetlinkHandle, cfg.InternalBridgeName)
}

func clearBridge(rootNetlinkHandle *remoteNetlink.Handle, bridgeName string) error {
	if err := clearLink(rootNetlinkHandle, bridgeName); err != nil {
		klog.ErrorS(err, "ClearBridge is failed", "bridgeName", bridgeName)
		return err
	}
	return nil
}

func ClearVethInterface(interfaceName string, isInternal bool) error {
	var err error
	var rootNetlinkHandle *remoteNetlink.Handle
	if rootNetlinkHandle, err = GetRootNetlinkHandle(); err != nil {
		klog.ErrorS(err, "Initializing failed while getting rootNetlinkHandle")
		return err
	}
	if isInternal {
		return clearVethInterface(rootNetlinkHandle, "int"+interfaceName)
	} else {
		return clearVethInterface(rootNetlinkHandle, "ext"+interfaceName)
	}
}

func clearVethInterface(rootNetlinkHandle *remoteNetlink.Handle, interfaceName string) error {
	if err := clearLink(rootNetlinkHandle, interfaceName); err != nil {
		klog.ErrorS(err, "ClearVethInterface is failed", "interfaceName", interfaceName)
		return err
	}
	klog.InfoS("ClearLink Done", "linkName", interfaceName)
	return nil
}

func clearLink(rootNetlinkHandle *remoteNetlink.Handle, linkName string) error {
	var err error
	la := remoteNetlink.NewLinkAttrs()
	la.Name = linkName

	link, err := rootNetlinkHandle.LinkByName(linkName)
	if err != nil {
		switch err.(type) {
		case remoteNetlink.LinkNotFoundError:
			klog.Warningf("Link not found. Skip clearLink() link:%s", linkName)
			return nil
		default:
			klog.ErrorS(err, "LinkByName failed")
			return err
		}
	}
	klog.InfoS("Link Detected Success", "linkName", linkName)

	if err := rootNetlinkHandle.LinkDel(link); err != nil {
		klog.ErrorS(err, "Clearing link failed", "linkName", linkName)
		return err
	}

	klog.InfoS("Clear link done", "linkName", linkName)
	return nil
}

func SetRoute2Container(containerPid int, interfaceName string, cidr string) error {
	// var vethPeerIntf remoteNetlink.Link
	var targetNetlinkHandle *remoteNetlink.Handle
	// var newinterfaceName string

	if netlinkHandle, err := GetTargetNetlinkHandle(GetNsHandle(CrioType(containerPid))); err != nil {
		klog.ErrorS(err, "GetTargetNetlinkHandle")
		return err
	} else {
		targetNetlinkHandle = netlinkHandle
	}

	rule := remoteNetlink.NewRule()
	rule.Mark = 200
	rule.Table = 200
	// _, srcIpv4Net, err := net.ParseCIDR(cidr)
	// if err != nil {
	// 	return fmt.Errorf("parse fail : %s", err.Error())
	// }
	// rule.Src = srcIpv4Net

	if ruleList, err := targetNetlinkHandle.RuleList(0); err != nil {
		klog.Error(err)
	} else {
		for _, v := range ruleList {
			if v.Table == 200 {
				if err := targetNetlinkHandle.RuleDel(&v); err != nil {
					klog.Error(err)
				} else {
					klog.InfoS("RuleDel Done", "rule", rule)
				}
			}
		}
	}

	if err := targetNetlinkHandle.RuleAdd(rule); err != nil {
		klog.Error(err)
	} else {
		klog.InfoS("RuleAdd Done", "rule", rule)
	}

	var externalIntf remoteNetlink.Link
	if link, err := targetNetlinkHandle.LinkByName(DefaultExternalContainerInterface); err != nil {
		klog.ErrorS(err, "Failed LinkByName", "interfaceName", DefaultExternalContainerInterface)
		return err
	} else {
		externalIntf = link
	}

	var internalIntf remoteNetlink.Link
	if link, err := targetNetlinkHandle.LinkByName(DefaultInternalContainerInterface); err != nil {
		klog.ErrorS(err, "Failed LinkByName", "interfaceName", DefaultInternalContainerInterface)
		return err
	} else {
		internalIntf = link
	}

	dst := &net.IPNet{
		IP:   net.IPv4(0, 0, 0, 0),
		Mask: net.CIDRMask(0, 32),
	}

	if err := targetNetlinkHandle.RouteAdd(&remoteNetlink.Route{
		Table:     200,
		Dst:       dst,
		LinkIndex: externalIntf.Attrs().Index,
		Gw:        originSnapshot.defaultGW,
	}); err != nil {
		klog.Error(err)
	}

	if routeList, err := targetNetlinkHandle.RouteList(externalIntf, 0); err != nil {
		klog.ErrorS(err, "Failed RouteList", "interfaceName", externalIntf)
		return err
	} else {
		for _, v := range routeList {
			if err := targetNetlinkHandle.RouteAdd(&remoteNetlink.Route{
				Table:     200,
				Dst:       v.Dst,
				Scope:     v.Scope,
				Src:       v.Src,
				LinkIndex: v.LinkIndex,
			}); err != nil {
				klog.Error(err)
			}
		}
	}

	if routeList, err := targetNetlinkHandle.RouteList(internalIntf, 0); err != nil {
		klog.ErrorS(err, "Failed RouteList", "interfaceName", internalIntf)
		return err
	} else {
		for _, v := range routeList {
			if err := targetNetlinkHandle.RouteAdd(&remoteNetlink.Route{
				Table:     200,
				Dst:       v.Dst,
				Scope:     v.Scope,
				Src:       v.Src,
				LinkIndex: v.LinkIndex,
			}); err != nil {
				klog.Error(err)
			}
		}
	}

	return nil
}

func PBR() error {

	rule := remoteNetlink.NewRule()
	rule.Table = 200
	_, srcIpv4Net, err := net.ParseCIDR("200.200.200.200/32")
	if err != nil {
		return fmt.Errorf("parse fail : %s", err.Error())
	}
	_, dstIpv4Net, err := net.ParseCIDR("200.200.200.201/32")
	if err != nil {
		return fmt.Errorf("parse fail : %s", err.Error())
	}
	rule.Src = srcIpv4Net
	rule.Dst = dstIpv4Net

	// if err := netlink.RuleAdd(rule); err != nil {
	// 	klog.Error(err)
	// }
	if err := remoteNetlink.RuleDel(rule); err != nil {
		klog.Error(err)
	}

	dst := &net.IPNet{
		IP:   net.IPv4(0, 0, 0, 0),
		Mask: net.CIDRMask(0, 32),
	}
	gw := net.IPv4(192, 168, 9, 1)
	// if err := netlink.RouteAdd(&remoteNetlink.Route{
	// 	Table: 200,
	// 	Dst:   dst,
	// 	Gw:    gw,
	// }); err != nil {
	// 	klog.Error(err)
	// }
	if err := remoteNetlink.RouteDel(&remoteNetlink.Route{
		Table: 200,
		Dst:   dst,
		Gw:    gw,
	}); err != nil {
		klog.Error(err)
	}

	return nil
}

func SetIPaddress2Container(containerPid int, interfaceName string, cidrs []string, isInternal bool, cfg *Config) error {
	var vethPeerIntf remoteNetlink.Link
	var targetNetlinkHandle *remoteNetlink.Handle
	var newinterfaceName string

	if netlinkHandle, err := GetTargetNetlinkHandle(GetNsHandle(CrioType(containerPid))); err != nil {
		klog.ErrorS(err, "GetTargetNetlinkHandle")
		return err
	} else {
		targetNetlinkHandle = netlinkHandle
	}

	if isInternal {
		newinterfaceName = "ethint"
	} else {
		newinterfaceName = "ethext"
	}

	if link, err := targetNetlinkHandle.LinkByName(newinterfaceName); err != nil {
		klog.ErrorS(err, "LinkByName is failed", "interfaceName", newinterfaceName+"1")
		return err
	} else {
		vethPeerIntf = link
	}

	if l, err := targetNetlinkHandle.AddrList(vethPeerIntf, remoteNetlink.FAMILY_ALL); err != nil {
		klog.ErrorS(err, "Listing Address failed", "interfaceName", vethPeerIntf.Attrs().Name)
		return err
	} else {
		for _, addr := range l {
			if err := targetNetlinkHandle.AddrDel(vethPeerIntf, &addr); err != nil {
				klog.ErrorS(err, "Deleting address failed", "interfaceName", vethPeerIntf.Attrs().Name, "address", addr.String())
				return err
			}
		}
	}

	for _, cidr := range cidrs {
		if a, err := remoteNetlink.ParseAddr(cidr); err != nil {
			klog.ErrorS(err, "ParseAddr is failed", "addr", cidr)
			return err
		} else {
			if err := targetNetlinkHandle.AddrAdd(vethPeerIntf, a); err != nil {
				klog.ErrorS(err, "AddrAdd is failed", "interfaceName", vethPeerIntf.Attrs().Name, "addr", a.IPNet.String())
				return err
			} else {
				klog.InfoS("AddrAdd is done", "interfaceName", vethPeerIntf.Attrs().Name, "addr", a.IPNet.String())
			}
		}
	}

	if err := setLinkUp(targetNetlinkHandle, vethPeerIntf); err != nil {
		return err
	}

	return nil
}

func SetInterface2Container(containerPid int, interfaceName string, isInternal bool, cfg *Config) error {
	var rootNetlinkHandle *remoteNetlink.Handle
	var err error

	if rootNetlinkHandle, err = GetRootNetlinkHandle(); err != nil {
		klog.ErrorS(err, "Initializing failed while getting rootNetlinkHandle")
		return err
	}

	var vethIntf remoteNetlink.Link
	var vethPeerIntf remoteNetlink.Link
	var newinterfaceName string
	var newinterfacePeerName string
	var targetNetlinkHandle *remoteNetlink.Handle
	var bridgeIntf remoteNetlink.Link
	var bridgeName string

	if isInternal {
		bridgeName = cfg.InternalBridgeName
		newinterfaceName = "int" + interfaceName
		newinterfacePeerName = "ethint"
	} else {
		bridgeName = cfg.ExternalBridgeName
		newinterfaceName = "ext" + interfaceName
		newinterfacePeerName = "ethext"
	}

	// if _, err := remoteNetlink.LinkByName(newinterfaceName + "0"); err == nil {
	if _, err := remoteNetlink.LinkByName(newinterfaceName); err == nil {
		return nil
	}

	veth := &remoteNetlink.Veth{
		LinkAttrs: remoteNetlink.LinkAttrs{
			Name: newinterfaceName,
		},
		PeerName: newinterfacePeerName,
		// PeerHardwareAddr: originLink.Attrs().HardwareAddr,
	}

	// if link, peerLink, err := SetVethInterface(rootNetlinkHandle, newinterfaceName); err != nil {
	if link, err := setLinkbyLink(rootNetlinkHandle, veth); err != nil {
		return err
	} else {
		vethIntf = link
		originSnapshot.newIntIfname = append(originSnapshot.newIntIfname, vethIntf.Attrs().Name)
	}

	if link, err := rootNetlinkHandle.LinkByName(veth.PeerName); err != nil {
		klog.ErrorS(err, "LinkByName is failed", "interfaceName", veth.PeerName)
		return err
	} else {
		vethPeerIntf = link
	}

	if netlinkHandle, err := GetTargetNetlinkHandle(GetNsHandle(CrioType(containerPid))); err != nil {
		klog.ErrorS(err, "GetTargetNetlinkHandle")
		return err
	} else {
		targetNetlinkHandle = netlinkHandle
	}

	if err := rootNetlinkHandle.LinkSetNsFd(vethPeerIntf, int(GetNsHandle(CrioType(containerPid)))); err != nil {
		klog.ErrorS(err, "Setting Veth interface to target NS failed", "interfaceName", vethPeerIntf.Attrs().Name, "targetNS fd value", int(GetNsHandle(CrioType(containerPid))))
	}

	if link, err := rootNetlinkHandle.LinkByName(bridgeName); err != nil {
		klog.ErrorS(err, "LinkByName is failed", "interfaceName", bridgeName)
		return err
	} else {
		bridgeIntf = link
	}

	if err := attachInterface2Bridge(rootNetlinkHandle, vethIntf, bridgeIntf); err != nil {
		klog.ErrorS(err, "attach failed", "interfaceName", vethIntf.Attrs().Name, "bridgeName", bridgeName)
		return err
	}

	if err := setLinkUp(rootNetlinkHandle, vethIntf); err != nil {
		return err
	}

	if err := setLinkUp(targetNetlinkHandle, vethPeerIntf); err != nil {
		return err
	}

	return nil

}

func SetVethInterface(rootNetlinkHandle *remoteNetlink.Handle, interfaceName string) (remoteNetlink.Link, remoteNetlink.Link, error) {
	if link, err := setLink(rootNetlinkHandle, interfaceName, TYPEVETH); err != nil {
		klog.ErrorS(err, "setVethInterface failed")
		return nil, nil, err
	} else {
		klog.Info("setVethInterface Done")
		if peerLink, err := remoteNetlink.LinkByName(interfaceName + "1"); err != nil {
			klog.ErrorS(err, "Get peer interface failed", "interfaceName", interfaceName+"1")
			return nil, nil, err
		} else {
			return link, peerLink, nil
		}
	}
}

func setExternalBridge(rootNetlinkHandle *remoteNetlink.Handle, cfg *Config) (remoteNetlink.Link, error) {
	if cfg.ExternalBridgeName == "" {
		return setBridge(rootNetlinkHandle, DefaultExternalBridgeName)
	}
	return setBridge(rootNetlinkHandle, cfg.ExternalBridgeName)
}

func setInternalBridge(rootNetlinkHandle *remoteNetlink.Handle, cfg *Config) (remoteNetlink.Link, error) {
	if cfg.InternalBridgeName == "" {
		return setBridge(rootNetlinkHandle, DefaultInternalBridgeName)
	}
	return setBridge(rootNetlinkHandle, cfg.InternalBridgeName)
}

func setBridge(rootNetlinkHandle *remoteNetlink.Handle, bridgeName string) (remoteNetlink.Link, error) {
	if link, err := remoteNetlink.LinkByName(bridgeName); err != nil {
		switch err.(type) {
		case remoteNetlink.LinkNotFoundError:
			break
		default:
			klog.ErrorS(err, "LinkByName failed")
			return nil, err
		}
	} else {
		return link, nil
	}

	if link, err := setLink(rootNetlinkHandle, bridgeName, TYPEBRIDGE); err != nil {
		klog.ErrorS(err, "setBridge failed")
		return nil, err
	} else {
		klog.Info("setBridge Done")
		return link, nil
	}
}

func setLinkbyLink(netlinkHandle *remoteNetlink.Handle, link remoteNetlink.Link) (remoteNetlink.Link, error) {
	if link, err := remoteNetlink.LinkByName(link.Attrs().Name); err != nil {
		switch err.(type) {
		case remoteNetlink.LinkNotFoundError:
			break
		default:
			klog.ErrorS(err, "LinkByName failed")
			return nil, err
		}
	} else {
		return link, nil
	}

	if err := netlinkHandle.LinkAdd(link); err != nil {
		klog.ErrorS(err, "Link add failed", "Link name", link.Attrs().Name)
		return nil, err
	}
	klog.InfoS("LinkAdd done", "Link name", link.Attrs().Name)

	return link, nil
}

func setLink(netlinkHandle *remoteNetlink.Handle, linkName string, linkType string) (remoteNetlink.Link, error) {
	if linkType == TYPEVETH {
		if link, err := remoteNetlink.LinkByName(linkName + "0"); err != nil {
			switch err.(type) {
			case remoteNetlink.LinkNotFoundError:
				break
			default:
				klog.ErrorS(err, "LinkByName failed")
				return nil, err
			}
		} else {
			return link, nil
		}
	} else {
		if link, err := remoteNetlink.LinkByName(linkName); err != nil {
			switch err.(type) {
			case remoteNetlink.LinkNotFoundError:
				break
			default:
				klog.ErrorS(err, "LinkByName failed")
				return nil, err
			}
		} else {
			return link, nil
		}
	}

	var err error
	la := remoteNetlink.NewLinkAttrs()
	la.Name = linkName

	exist := false

	_, err = netlinkHandle.LinkByName(la.Name)
	if err != nil {
		if err.Error() == "Link not found" {
			exist = false
		} else {
			klog.Error(err)
			return nil, err
		}
	} else {
		exist = true
	}

	var link remoteNetlink.Link
	switch linkType {
	case TYPEBRIDGE:
		link = &remoteNetlink.Bridge{LinkAttrs: la, VlanFiltering: &[]bool{true}[0]}
	case TYPEVETH:
		la.Name += "0"
		link = &remoteNetlink.Veth{LinkAttrs: la, PeerName: linkName + "1"}
	}

	if !exist {
		err = netlinkHandle.LinkAdd(link)
		if err != nil {
			klog.ErrorS(err, "Link add failed", "Link name", la.Name, "Link type", linkType)
			return nil, err
		}
		klog.InfoS("LinkAdd done", "Link name", la.Name, "Link type", linkType)
	}

	return link, nil
}

func setLinkUp(netlinkHandle *remoteNetlink.Handle, link remoteNetlink.Link) error {
	return netlinkHandle.LinkSetUp(link)
}

func setLinkDown(netlinkHandle *remoteNetlink.Handle, link remoteNetlink.Link) error {
	return netlinkHandle.LinkSetDown(link)
}

func GetRootNetlinkHandle() (*remoteNetlink.Handle, error) {
	handle, err := remoteNetlink.NewHandle()
	if err != nil {
		klog.ErrorS(err, "Error occured while geting RootNSHandle")
		return nil, err
	}
	return handle, nil
}

func GetTargetNetlinkHandle(ns netns.NsHandle) (*remoteNetlink.Handle, error) {
	if ns == 0 {
		klog.Error("Getting wrong ns number(0)")
		return nil, fmt.Errorf("getting wrong ns number(0)")
	}

	var handle *remoteNetlink.Handle
	var err error

	handle, err = remoteNetlink.NewHandleAt(ns)
	if err != nil {
		klog.ErrorS(err, "Retrieviing Handle from ns is failed", "ns", ns)
		return nil, err
	}
	return handle, nil
}

func GetNsHandle(arg interface{}) netns.NsHandle {
	switch arg.(type) {

	case DockerType:
		handle, err := netns.GetFromDocker(string(arg.(DockerType)))
		if err != nil {
			klog.ErrorS(err, "Getting NsHandle Docker", "ns", string(arg.(DockerType)))
			return 0
		}
		return handle
	case CrioType:
		handle, err := netns.GetFromPid(int(arg.(CrioType)))
		if err != nil {
			klog.ErrorS(err, "Getting NsHandle Crio", "ns", int(arg.(CrioType)))
			return 0
		}
		return handle
	case string:
		if arg.(string) == "root" {
			handle, err := netns.Get()
			if err != nil {
				klog.ErrorS(err, "Getting NsHandle Docker", "ns", string(arg.(string)))
				return 0
			}
			return handle

		} else {
			handle, err := netns.GetFromName(arg.(string))
			if err != nil {
				klog.ErrorS(err, "Getting NsHandle Docker", "ns", string(arg.(DockerType)))
				return 0
			}
			return handle
		}
	}

	return 0
}

func SetVlan(interfaceName string, newVlan int, oldVlan int, cfg *Config) error {
	var rootNetlinkHandle *remoteNetlink.Handle
	var err error

	if rootNetlinkHandle, err = GetRootNetlinkHandle(); err != nil {
		klog.ErrorS(err, "Initializing failed while getting rootNetlinkHandle")
		return err
	}

	if oldVlan != 0 {
		if err := delVlan(rootNetlinkHandle, interfaceName, oldVlan, true, true); err != nil {
			return err
		}
		if err := delVlan(rootNetlinkHandle, originSnapshot.intIfname, oldVlan, false, false); err != nil {
			return err
		}
	}
	if newVlan != 0 {
		if err := addVlan(rootNetlinkHandle, interfaceName, newVlan, true, true); err != nil {
			return err
		}
		if err := addVlan(rootNetlinkHandle, originSnapshot.intIfname, newVlan, false, false); err != nil {
			return err
		}
	}

	return nil
}

func addVlan(rootNetlinkHandle *remoteNetlink.Handle, interfaceName string, vlan int, pvid bool, untagged bool) error {
	var intf remoteNetlink.Link
	if link, err := remoteNetlink.LinkByName(interfaceName); err != nil {
		return err
	} else {
		intf = link
	}

	if err := rootNetlinkHandle.BridgeVlanAdd(intf, uint16(vlan), pvid, untagged, false, true); err != nil {
		klog.ErrorS(err, "BridgeVlanAdd failed", "interfaceName", intf.Attrs().Name, "vlan", vlan)
		return err
	}
	return nil
}

func delVlan(rootNetlinkHandle *remoteNetlink.Handle, interfaceName string, vlan int, pvid bool, untagged bool) error {
	var intf remoteNetlink.Link
	if link, err := remoteNetlink.LinkByName(interfaceName); err != nil {
		switch err.(type) {
		case remoteNetlink.LinkNotFoundError:
			return nil
		default:
			return err
		}
	} else {
		intf = link
	}

	if err := rootNetlinkHandle.BridgeVlanDel(intf, uint16(vlan), pvid, untagged, false, true); err != nil {
		klog.ErrorS(err, "BridgeVlanDel failed", "interfaceName", intf.Attrs().Name, "vlan", vlan)
		return err
	}
	return nil
}
