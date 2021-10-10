package daemon

import (
	"fmt"

	internalCrio "github.com/cho4036/virtualrouter-controller/internal/daemon/crio"
	internalNetlink "github.com/cho4036/virtualrouter-controller/internal/daemon/netlink"
	"k8s.io/klog/v2"
)

type NetworkDaemon struct {
	crioCfg     *internalCrio.CrioConfig
	netlinkCfg  *internalNetlink.Config
	runnigState map[string]*virtualrouterSpec
}

type virtualrouterSpec struct {
	vlan        int
	internalIPs []string
	externalIPs []string
}

func NewDaemon(crioCfg *internalCrio.CrioConfig, netlinkCfg *internalNetlink.Config) *NetworkDaemon {
	return &NetworkDaemon{
		crioCfg:     crioCfg,
		netlinkCfg:  netlinkCfg,
		runnigState: make(map[string]*virtualrouterSpec),
	}
}

func (n *NetworkDaemon) Initialize() error {
	if err := internalCrio.Initialize(n.crioCfg); err != nil {
		klog.ErrorS(err, "Crio Initialization failed")
		return err
	}

	if err := internalNetlink.Initialize(n.netlinkCfg); err != nil {
		klog.ErrorS(err, "Netlink Initialization failed")
		return err
	}
	return nil
}

func (n *NetworkDaemon) ClearAll() error {
	if err := internalNetlink.Clear(n.netlinkCfg); err != nil {
		klog.ErrorS(err, "Netlink Clear failed")
		return err
	}
	return nil
}

func (n *NetworkDaemon) Sync(containerName string, vlan int, internalIPs []string, externalIPs []string) error {
	// var vlanChanged, internalIPsChanged, externalIPsChanged bool

	// if val, exist := n.runnigState[containerName]; exist {
	// 	if val.vlan != vlan {
	// 		vlanChanged = true
	// 	}

	// } else {
	// 	n.runnigState[containerName] = &virtualrouterSpec{
	// 		vlan:        vlan,
	// 		internalIPs: internalIPs,
	// 		externalIPs: externalIPs,
	// 	}
	// }
	if len(internalIPs) == 0 {
	} else {
		n.ConnectInterface(containerName, true)
		n.AssignIPaddress(containerName, internalIPs, true)
	}

	if len(externalIPs) == 0 {
	} else {
		n.ConnectInterface(containerName, false)
		n.AssignIPaddress(containerName, internalIPs, false)
	}

	return nil
}

func (n *NetworkDaemon) AssignIPaddress(containerName string, cidrs []string, isInternal bool) error {
	var containerID string
	var containerPid int

	containerID = internalCrio.GetContainerIDFromContainerName(containerName, n.crioCfg)
	if containerID == "" {
		klog.Errorf("There is no running container with ContainerName: %s", containerName)
		return fmt.Errorf("no running container found")
	}

	containerPid = internalCrio.GetContainerPid(containerID)
	if containerPid <= 0 {
		klog.Errorf("Wrong Pid(%d) value of Container(%s)", containerPid, containerName)
		return fmt.Errorf("internal error")
	}

	if err := internalNetlink.SetIPaddress2Container(containerPid, containerID[:7], cidrs, isInternal, n.netlinkCfg); err != nil {
		klog.ErrorS(err, "Set Interface to Container failed", "ContainerName", containerName, "ContainerID", containerID)
		return err
	}

	return nil
}

func (n *NetworkDaemon) ConnectInterface(containerName string, isInternal bool) error {
	var containerID string
	var containerPid int

	containerID = internalCrio.GetContainerIDFromContainerName(containerName, n.crioCfg)
	if containerID == "" {
		klog.Errorf("There is no running container with ContainerName: %s", containerName)
		return fmt.Errorf("no running container found")
	}

	containerPid = internalCrio.GetContainerPid(containerID)
	if containerPid <= 0 {
		klog.Errorf("Wrong Pid(%d) value of Container(%s)", containerPid, containerName)
		return fmt.Errorf("internal error")
	}

	if err := internalNetlink.SetInterface2Container(containerPid, containerID[:7], isInternal, n.netlinkCfg); err != nil {
		klog.ErrorS(err, "Set Interface to Container failed", "ContainerName", containerName, "ContainerID", containerID)
		return err
	}
	return nil
}
