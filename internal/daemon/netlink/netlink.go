package netlink

import (
	"context"
	"fmt"
	"net"
	"strconv"

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

	podIntIfName []string
	podExtIfName []string

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
		podIntIfName: make([]string, 0),
		podExtIfName: make([]string, 0),
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

	var defaultGW net.IP = getDefaultGW()

	if err := initInternalInterface(rootNetlinkHandle, cfg); err != nil {
		klog.ErrorS(err, "Initializing failed while setting InternalInterface")
	}

	if err := initExternalInterface(rootNetlinkHandle, cfg); err != nil {
		klog.ErrorS(err, "Initializing failed while setting InternalInterface")
	}

	if defaultGW == nil {
		return nil
	} else {
		if err := setDefaultGW(rootNetlinkHandle, defaultGW); err != nil {
			klog.ErrorS(err, "setDefaultGW failed", "gw", defaultGW)
			return err
		} else {
			klog.InfoS("setDefaultGW done", "gw", defaultGW)
		}
	}

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

	for _, newIntInterface := range originSnapshot.podIntIfName {
		if err := clearVethInterface(rootNetlinkHandle, newIntInterface); err != nil {
			klog.ErrorS(err, "Clearing failed while deleting Internal Interfae")
			return err
		}
	}

	for _, newExtInterface := range originSnapshot.podExtIfName {
		if err := clearVethInterface(rootNetlinkHandle, newExtInterface); err != nil {
			klog.ErrorS(err, "Clearing failed while deleting External Interface")
			return err
		}
	}

	if err := setDefaultGW(rootNetlinkHandle, originSnapshot.defaultGW); err != nil {
		klog.ErrorS(err, "setDefaultGW failed", "gw", originSnapshot.defaultGW)
		return err
	}

	return nil
}

func initInternalInterface(rootNetlinkHandle *remoteNetlink.Handle, cfg *Config) error {
	var originLink remoteNetlink.Link
	var newLink, newPeerLink remoteNetlink.Link
	var newLinkName, newPeerLinkName = cfg.NewInternalInterfaceName + "0", cfg.NewInternalInterfaceName + "1"
	var err error

	if originLink, err = rootNetlinkHandle.LinkByName(cfg.OriginInternalInterfaceName); err != nil {
		klog.ErrorS(err, "LinkByName is failed", "interfaceName", cfg.OriginInternalInterfaceName)
		return err
	}

	if newLink, err = rootNetlinkHandle.LinkByName(newLinkName); err != nil {
		veth := &remoteNetlink.Veth{
			LinkAttrs: remoteNetlink.LinkAttrs{
				Name: newLinkName,
			},
			PeerName: newPeerLinkName,
		}
		if link, err := setLink(rootNetlinkHandle, veth); err != nil {
			klog.ErrorS(err, "setLink failed", "interfaceName", veth.Attrs().Name)
			return err
		} else {
			newLink = link
		}
	}
	if newPeerLink, err = rootNetlinkHandle.LinkByName(newPeerLinkName); err != nil {
		return err
	}

	var originNetAddrs []net.Addr

	if ifaces, err := net.Interfaces(); err != nil {
		klog.ErrorS(err, "Listing Interfaces failed")
		return err
	} else {
		for _, i := range ifaces {
			if i.Name == cfg.OriginInternalInterfaceName {
				originNetAddrs, _ = i.Addrs()
			}
		}
	}

	var bridgeLink remoteNetlink.Link

	if link, err := rootNetlinkHandle.LinkByName(cfg.InternalBridgeName); err != nil {
		klog.ErrorS(err, "LinkByName is failed", "interfaceName", cfg.InternalBridgeName)
		return err
	} else {
		bridgeLink = link
	}

	if err := attachInterface2Bridge(rootNetlinkHandle, originLink, bridgeLink); err != nil {
		klog.ErrorS(err, "attachInterface2Bridge is failed", "interfaceName", originLink.Attrs().Name, "bridgeName", bridgeLink.Attrs().Name)
		return err
	}

	if err := attachInterface2Bridge(rootNetlinkHandle, newLink, bridgeLink); err != nil {
		klog.ErrorS(err, "attachInterface2Bridge is failed", "interfaceName", newLink.Attrs().Name, "bridgeName", bridgeLink.Attrs().Name)
		return err
	}

	for _, addr := range originNetAddrs {
		if a, err := remoteNetlink.ParseAddr(addr.String()); err != nil {
			klog.ErrorS(err, "ParseAddr is failed", "addr", addr.String())
			return err
		} else {
			if err := rootNetlinkHandle.AddrDel(originLink, a); err != nil {
				klog.ErrorS(err, "AddrDel is failed", "Link Name", newPeerLink.Attrs().Name, "addr", a.IPNet.String())
				return err
			} else {
				klog.InfoS("AddrDel is done", "Link Name", newPeerLink.Attrs().Name, "addr", a.IPNet.String())
			}
			if err := rootNetlinkHandle.AddrAdd(newPeerLink, a); err != nil {
				klog.ErrorS(err, "AddrAdd is failed", "Link Name", newPeerLink.Attrs().Name, "addr", a.IPNet.String())
				return err
			} else {
				klog.InfoS("AddrAdd is done", "Link Name", newPeerLink.Attrs().Name, "addr", a.IPNet.String())
			}
		}
	}

	if err := setLinkUp(rootNetlinkHandle, newLink); err != nil {
		klog.ErrorS(err, "setLinkUp is failed", "interfaceName", newLink.Attrs().Name)
		return err
	}
	if err := setLinkUp(rootNetlinkHandle, newPeerLink); err != nil {
		klog.ErrorS(err, "setLinkUp is failed", "interfaceName", newPeerLink.Attrs().Name)
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
	var originLink remoteNetlink.Link
	var newLink, newPeerLink remoteNetlink.Link
	var newLinkName, newPeerLinkName = cfg.NewExternalInterfaceName + "0", cfg.NewExternalInterfaceName + "1"
	var err error

	if originLink, err = rootNetlinkHandle.LinkByName(cfg.OriginExternalInterfaceName); err != nil {
		klog.ErrorS(err, "LinkByName is failed", "interfaceName", cfg.OriginExternalInterfaceName)
		return err
	}

	if newLink, err = rootNetlinkHandle.LinkByName(newLinkName); err != nil {
		veth := &remoteNetlink.Veth{
			LinkAttrs: remoteNetlink.LinkAttrs{
				Name: newLinkName,
			},
			PeerName: newPeerLinkName,
		}
		if link, err := setLink(rootNetlinkHandle, veth); err != nil {
			klog.ErrorS(err, "setLink failed", "interfaceName", veth.Attrs().Name)
			return err
		} else {
			newLink = link
		}
	}
	if newPeerLink, err = rootNetlinkHandle.LinkByName(newPeerLinkName); err != nil {
		return err
	}

	var originNetAddrs []net.Addr

	if ifaces, err := net.Interfaces(); err != nil {
		klog.ErrorS(err, "Listing Interfaces failed")
		return err
	} else {
		for _, i := range ifaces {
			if i.Name == cfg.OriginExternalInterfaceName {
				originNetAddrs, _ = i.Addrs()
			}
		}
	}

	var bridgeLink remoteNetlink.Link
	if link, err := rootNetlinkHandle.LinkByName(cfg.ExternalBridgeName); err != nil {
		klog.ErrorS(err, "LinkByName is failed", "interfaceName", cfg.ExternalBridgeName)
		return err
	} else {
		bridgeLink = link
	}

	if err := attachInterface2Bridge(rootNetlinkHandle, originLink, bridgeLink); err != nil {
		klog.ErrorS(err, "attachInterface2Bridge is failed", "interfaceName", originLink.Attrs().Name, "bridgeName", bridgeLink.Attrs().Name)
		return err
	}

	if err := attachInterface2Bridge(rootNetlinkHandle, newLink, bridgeLink); err != nil {
		klog.ErrorS(err, "attachInterface2Bridge is failed", "interfaceName", newLink.Attrs().Name, "bridgeName", bridgeLink.Attrs().Name)
		return err
	}

	for _, addr := range originNetAddrs {
		if a, err := remoteNetlink.ParseAddr(addr.String()); err != nil {
			klog.ErrorS(err, "ParseAddr is failed", "addr", addr.String())
			return err
		} else {
			if err := rootNetlinkHandle.AddrDel(originLink, a); err != nil {
				klog.ErrorS(err, "AddrDel is failed", "Link Name", newPeerLink.Attrs().Name, "addr", a.IPNet.String())
				return err
			} else {
				klog.InfoS("AddrDel is done", "Link Name", newPeerLink.Attrs().Name, "addr", a.IPNet.String())
			}
			if err := rootNetlinkHandle.AddrAdd(newPeerLink, a); err != nil {
				klog.ErrorS(err, "AddrAdd is failed", "Link Name", newPeerLink.Attrs().Name, "addr", a.IPNet.String())
				return err
			} else {
				klog.InfoS("AddrAdd is done", "Link Name", newPeerLink.Attrs().Name, "addr", a.IPNet.String())
			}
		}
	}

	if err := setLinkUp(rootNetlinkHandle, newLink); err != nil {
		klog.ErrorS(err, "setLinkUp is failed", "interfaceName", newLink.Attrs().Name)
		return err
	}
	if err := setLinkUp(rootNetlinkHandle, newPeerLink); err != nil {
		klog.ErrorS(err, "setLinkUp is failed", "interfaceName", newPeerLink.Attrs().Name)
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

func getDefaultGW() net.IP {
	routes, _ := remoteNetlink.RouteListFiltered(remoteNetlink.FAMILY_V4, &remoteNetlink.Route{
		Dst: nil,
	}, remoteNetlink.RT_FILTER_DST)
	if len(routes) == 0 {
		return nil
	}
	return routes[0].Gw
}

func setDefaultGW(rootNetlinkHandle *remoteNetlink.Handle, gw net.IP) error {
	routes, _ := remoteNetlink.RouteListFiltered(remoteNetlink.FAMILY_V4, &remoteNetlink.Route{
		Dst: nil,
	}, remoteNetlink.RT_FILTER_DST)
	for _, route := range routes {
		rootNetlinkHandle.RouteDel(&route)
	}

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

func SetRouteRule2Container(containerPid int, markNumber int, tableNumber int) error {
	var targetNetlinkHandle *remoteNetlink.Handle

	if netlinkHandle, err := GetTargetNetlinkHandle(GetNsHandle(CrioType(containerPid))); err != nil {
		klog.ErrorS(err, "GetTargetNetlinkHandle")
		return err
	} else {
		targetNetlinkHandle = netlinkHandle
	}

	rule := remoteNetlink.NewRule()
	rule.Mark = markNumber
	rule.Table = tableNumber

	if ruleList, err := targetNetlinkHandle.RuleList(0); err != nil {
		klog.Error(err)
	} else {
		for _, v := range ruleList {
			if v.Table == tableNumber && v.Mark == markNumber {
				return nil
			}
		}
	}

	if err := targetNetlinkHandle.RuleAdd(rule); err != nil {
		klog.Error(err)
	} else {
		klog.InfoS("RuleAdd Done", "rule", rule)
	}

	return nil
}

func SetDefaultRoute2Container(containerPid int, gwIP string, tableNum int) error {
	var targetNetlinkHandle *remoteNetlink.Handle
	var err error
	// var newinterfaceName string

	if targetNetlinkHandle, err = GetTargetNetlinkHandle(GetNsHandle(CrioType(containerPid))); err != nil {
		klog.ErrorS(err, "GetTargetNetlinkHandle")
		return err
	}

	routes, _ := targetNetlinkHandle.RouteListFiltered(remoteNetlink.FAMILY_V4, &remoteNetlink.Route{
		Table: tableNum,
		Dst:   nil,
	}, remoteNetlink.RT_FILTER_DST|remoteNetlink.RT_FILTER_TABLE)
	for _, route := range routes {
		if route.Dst == nil && route.Table == tableNum {
			targetNetlinkHandle.RouteDel(&route)
		}
	}

	if err := targetNetlinkHandle.RouteAdd(&remoteNetlink.Route{
		Table: tableNum,
		Dst:   nil,
		Gw:    net.ParseIP(gwIP),
	}); err != nil {
		klog.Error(err)
	}

	return nil
}

func SetRoute2Container(containerPid int, interfaceName string, tableNum int) error {
	var targetNetlinkHandle *remoteNetlink.Handle
	var err error

	if targetNetlinkHandle, err = GetTargetNetlinkHandle(GetNsHandle(CrioType(containerPid))); err != nil {
		klog.ErrorS(err, "GetTargetNetlinkHandle")
		return err
	}

	var targetInterface remoteNetlink.Link
	if link, err := targetNetlinkHandle.LinkByName(interfaceName); err != nil {
		klog.ErrorS(err, "Failed LinkByName", "interfaceName", interfaceName)
		return err
	} else {
		targetInterface = link
	}

	if routeList, err := targetNetlinkHandle.RouteList(targetInterface, 0); err != nil {
		klog.ErrorS(err, "Failed RouteList", "interfaceName", targetInterface)
		return err
	} else {
		for _, v := range routeList {
			if v.Table == tableNum {
				targetNetlinkHandle.RouteDel(&v)
			}
		}
	}

	if routeList, err := targetNetlinkHandle.RouteList(targetInterface, 0); err != nil {
		klog.ErrorS(err, "Failed RouteList", "interfaceName", targetInterface)
		return err
	} else {
		for _, v := range routeList {
			if err := targetNetlinkHandle.RouteAdd(&remoteNetlink.Route{
				Table:     tableNum,
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

	if err := remoteNetlink.RuleDel(rule); err != nil {
		klog.Error(err)
	}

	dst := &net.IPNet{
		IP:   net.IPv4(0, 0, 0, 0),
		Mask: net.CIDRMask(0, 32),
	}
	gw := net.IPv4(192, 168, 9, 1)
	if err := remoteNetlink.RouteDel(&remoteNetlink.Route{
		Table: 200,
		Dst:   dst,
		Gw:    gw,
	}); err != nil {
		klog.Error(err)
	}

	return nil
}

func SetIPaddress2Container(containerPid int, ip string, netmask string, isInternal bool) error {
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

	ipMask, _ := net.IPMask(net.ParseIP(netmask).To4()).Size()
	ipAddr := ip + "/" + strconv.Itoa(ipMask)

	if a, err := remoteNetlink.ParseAddr(ipAddr); err != nil {
		klog.ErrorS(err, "ParseAddr is failed", "addr", ipAddr)
		return err
	} else {
		if err := targetNetlinkHandle.AddrAdd(vethPeerIntf, a); err != nil {
			klog.ErrorS(err, "AddrAdd is failed", "interfaceName", vethPeerIntf.Attrs().Name, "addr", a.IPNet.String())
			return err
		} else {
			klog.InfoS("AddrAdd is done", "interfaceName", vethPeerIntf.Attrs().Name, "addr", a.IPNet.String())
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
	if link, err := setLink(rootNetlinkHandle, veth); err != nil {
		if veth.PeerName[:3] == "eth" {
			if err := clearLink(rootNetlinkHandle, veth.PeerName); err != nil {
				klog.ErrorS(err, "ClearVethInterface is failed", "interfaceName", newinterfacePeerName)
				return err
			}
		}
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
	if link, err := rootNetlinkHandle.LinkByName(bridgeName); err != nil {
		switch err.(type) {
		case remoteNetlink.LinkNotFoundError:
			klog.InfoS("LinkNotFound", bridgeName)
		default:
			klog.ErrorS(err, "LinkByName failed")
			return nil, err
		}
	} else {
		klog.InfoS("Alreay exist link", link.Attrs().Name)
		return link, nil
	}

	bridge := &remoteNetlink.Bridge{
		LinkAttrs: remoteNetlink.LinkAttrs{
			Name: bridgeName,
		},
		VlanFiltering: &[]bool{true}[0],
	}

	if link, err := setLink(rootNetlinkHandle, bridge); err != nil {
		klog.ErrorS(err, "Failed setLink", "bridgeName", bridge.Attrs().Name)
		return nil, err
	} else {
		klog.Info("setBridge Done", "bridgeName", link.Attrs().Name)
		return link, nil
	}
}

func setLink(netlinkHandle *remoteNetlink.Handle, link remoteNetlink.Link) (remoteNetlink.Link, error) {
	if link, err := remoteNetlink.LinkByName(link.Attrs().Name); err != nil {
		switch err.(type) {
		case remoteNetlink.LinkNotFoundError:
			break
		default:
			klog.ErrorS(err, "LinkByName failed")
			return nil, err
		}
	} else {
		klog.InfoS("Already exist", link.Attrs().Name)
		return link, nil
	}

	if err := netlinkHandle.LinkAdd(link); err != nil {
		klog.ErrorS(err, "Link add failed", "Link name", link.Attrs().Name)
		return nil, err
	}
	klog.InfoS("LinkAdd done", "Link name", link.Attrs().Name)

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
		if err := delVlan(rootNetlinkHandle, cfg.OriginInternalInterfaceName, oldVlan, false, false); err != nil {
			return err
		}
	}
	if newVlan != 0 {
		if err := addVlan(rootNetlinkHandle, interfaceName, newVlan, true, true); err != nil {
			return err
		}
		if err := addVlan(rootNetlinkHandle, cfg.OriginInternalInterfaceName, newVlan, false, false); err != nil {
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
