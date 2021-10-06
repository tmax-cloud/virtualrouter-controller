package netlink

import (
	"context"
	"fmt"

	"github.com/vishvananda/netlink"
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
)

func Initialize(rootNetlinkHandle *remoteNetlink.Handle, cfg Config) error {
	if err := setExternalBridge(rootNetlinkHandle, &cfg); err != nil {
		klog.ErrorS(err, "Initializing failed while setting ExternalBridge")
		return err
	}

	if err := setInternalBridge(rootNetlinkHandle, &cfg); err != nil {
		klog.ErrorS(err, "Initializing failed while setting InternalBridge")
		return err
	}

	return nil
}

func Clear(rootNetlinkHandle *remoteNetlink.Handle, cfg Config) error {
	if err := clearExternalBridge(rootNetlinkHandle, &cfg); err != nil {
		klog.ErrorS(err, "Clearing failed while deleting ExternalBridge")
		return err
	}

	if err := clearInternalBridge(rootNetlinkHandle, &cfg); err != nil {
		klog.ErrorS(err, "Clearing failed while deleting InternalBridge")
		return err
	}

	return nil
}

// func GenerateVeth(containerID string, pid int) error {
// 	var rootNetlinkHandle *remoteNetlink.Handle
// 	var err error
// 	if rootNetlinkHandle, err = getRootNetlinkHandle(); err != nil {
// 		klog.ErrorS(err, "Initializing failed while getting rootNetlinkHandle")
// 		return err
// 	}

// 	// var err error

// 	// rootNetlinkHandle := getRootNetlinkHandle()
// 	// TargetNetlinkHandle := getTargetNetlinkHandle(getNsHandle(crioType(pid)))

// }

func SetVethToContainer(rootNetlinkHandle *remoteNetlink.Handle, targetNetlinkHandle *remoteNetlink.Handle, veth0 netlink.Link, veth1 netlink.Link) {

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
		return ClearBridge(rootNetlinkHandle, DefaultExternalBridgeName)
	}
	return ClearBridge(rootNetlinkHandle, cfg.ExternalBridgeName)
}

func clearInternalBridge(rootNetlinkHandle *remoteNetlink.Handle, cfg *Config) error {
	if cfg.InternalBridgeName == "" {
		return ClearBridge(rootNetlinkHandle, DefaultInternalBridgeName)
	}
	return ClearBridge(rootNetlinkHandle, cfg.InternalBridgeName)
}

func ClearBridge(rootNetlinkHandle *remoteNetlink.Handle, bridgeName string) error {
	var err error
	la := remoteNetlink.NewLinkAttrs()
	la.Name = bridgeName

	bridge, err := rootNetlinkHandle.LinkByName(bridgeName)
	if err != nil {
		if err.Error() == "bridge not found" {
			klog.InfoS("Bridge Detected Success", "bridgeName", bridgeName)
			return nil
		} else {
			klog.ErrorS(err, "Bridge Detection failed", "bridgeName", bridgeName)
			return err
		}
	}

	if err := rootNetlinkHandle.LinkDel(bridge); err != nil {
		klog.ErrorS(err, "Clearing Bridge failed", "bridgeName", bridgeName)
		return err
	}

	klog.InfoS("ClearBridge done", "bridgeName", bridgeName)
	return nil
}

func SetVethInterface(rootNetlinkHandle *remoteNetlink.Handle, interfaceName string) error {

	if err := setLink(rootNetlinkHandle, interfaceName, TYPEVETH); err != nil {
		klog.ErrorS(err, "setVethInterface failed")
		return err
	}
	klog.Info("setVethInterface Done")
	return nil
}

func setExternalBridge(rootNetlinkHandle *remoteNetlink.Handle, cfg *Config) error {
	if cfg.ExternalBridgeName == "" {
		return setBridge(rootNetlinkHandle, DefaultExternalBridgeName)
	}
	return setBridge(rootNetlinkHandle, cfg.ExternalBridgeName)
}

func setInternalBridge(rootNetlinkHandle *remoteNetlink.Handle, cfg *Config) error {
	if cfg.InternalBridgeName == "" {
		return setBridge(rootNetlinkHandle, DefaultInternalBridgeName)
	}
	return setBridge(rootNetlinkHandle, cfg.InternalBridgeName)
}

func setBridge(rootNetlinkHandle *remoteNetlink.Handle, bridgeName string) error {
	if err := setLink(rootNetlinkHandle, bridgeName, TYPEBRIDGE); err != nil {
		klog.ErrorS(err, "setBridge failed")
		return err
	}
	klog.Info("setBridge Done")
	return nil
}

func setLink(netlinkHandle *remoteNetlink.Handle, linkName string, linkType string) error {
	var err error
	la := netlink.NewLinkAttrs()
	la.Name = linkName + "0"

	exist := false

	_, err = netlinkHandle.LinkByName(la.Name)
	if err != nil {
		if err.Error() == "Link not found" {
			exist = false
		} else {
			klog.Error(err)
			return err
		}
	} else {
		exist = true
	}
	var link netlink.Link
	switch linkType {
	case TYPEBRIDGE:
		link = &netlink.Bridge{LinkAttrs: la, VlanFiltering: &[]bool{true}[0]}
	case TYPEVETH:
		link = &netlink.Veth{LinkAttrs: la, PeerName: linkName + "1"}
	}

	//// making bridge
	if !exist {
		err = netlinkHandle.LinkAdd(link)
		if err != nil {
			klog.ErrorS(err, "Link add failed", "Link name", la.Name, "Link type", linkType)
			return err
		}
		klog.InfoS("LinkAdd done", "Link name", la.Name, "Link type", linkType)
	}

	netlinkHandle.LinkSetUp(link)
	klog.InfoS("Link Set up done", "Link name", la.Name, "Link type", linkType)
	if err != nil {
		klog.Error(err)
		return err
	}

	return nil
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

func GetNsHandle(arg interface{}) (netns.NsHandle, error) {
	switch arg.(type) {

	case DockerType:
		handle, err := netns.GetFromDocker(string(arg.(DockerType)))
		if err != nil {
			klog.ErrorS(err, "Getting NsHandle Docker", "ns", string(arg.(DockerType)))
			return 0, err
		}
		return handle, nil
	case CrioType:
		handle, err := netns.GetFromPid(int(arg.(CrioType)))
		if err != nil {
			klog.ErrorS(err, "Getting NsHandle Crio", "ns", int(arg.(CrioType)))
			return 0, err
		}
		return handle, nil
	case string:
		if arg.(string) == "root" {
			handle, err := netns.Get()
			if err != nil {
				klog.ErrorS(err, "Getting NsHandle Docker", "ns", string(arg.(string)))
				return 0, err
			}
			return handle, nil

		} else {
			handle, err := netns.GetFromName(arg.(string))
			if err != nil {
				klog.ErrorS(err, "Getting NsHandle Docker", "ns", string(arg.(DockerType)))
				return 0, err
			}
			return handle, nil
		}
	}

	return 0, fmt.Errorf("wrong argument")
}
