package daemon

import (
	"fmt"
	"sort"

	internalCrio "github.com/tmax-cloud/virtualrouter-controller/internal/daemon/crio"
	internalNetlink "github.com/tmax-cloud/virtualrouter-controller/internal/daemon/netlink"
	"k8s.io/klog/v2"
)

type NetworkDaemon struct {
	crioCfg     *internalCrio.CrioConfig
	netlinkCfg  *internalNetlink.Config
	runnigState map[string]*virtualrouterSpec
	vlanUse     map[int][]string
}

type virtualrouterSpec struct {
	vlan         int
	internalIPs  []string
	externalIPs  []string
	internalCIDR string
}

func NewDaemon(crioCfg *internalCrio.CrioConfig, netlinkCfg *internalNetlink.Config) *NetworkDaemon {
	return &NetworkDaemon{
		crioCfg:     crioCfg,
		netlinkCfg:  netlinkCfg,
		runnigState: make(map[string]*virtualrouterSpec),
		vlanUse:     make(map[int][]string),
	}
}

func (n *NetworkDaemon) Start(stopSignalCh <-chan struct{}, stopCh chan<- struct{}) error {
	klog.Info("Starting NetworkDaemon")
	klog.Info("Initializing start")

	go func() {
		if err := n.Initialize(); err != nil {
			klog.Error("failed to Initialize")
		} else {
			klog.Info("Initializing done")
		}
		<-stopSignalCh
		klog.Info("ClearAll start")
		if err := n.ClearAll(); err != nil {
			klog.Error("failed to Clear")
		}
		klog.Info("ClearAll done")
		klog.Info("Shutting down Network Daemon")
		close(stopCh)
	}()

	return nil
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

func (n *NetworkDaemon) ClearContainer(containerName string) error {
	klog.InfoS("ClearContainer Start", "ContainerName", containerName)
	if _, exist := n.runnigState[containerName]; !exist {
		return nil
	}
	if n.runnigState[containerName].vlan != 0 {
		if err := n.AssignVlan(containerName, 0, n.runnigState[containerName].vlan); err != nil {
			return err
		}
	}

	containerID := internalCrio.GetContainerIDFromContainerName(containerName, n.crioCfg)
	if containerID == "" {
		klog.Errorf("There is no running container with ContainerName: %s", containerName)
		return fmt.Errorf("no running container found")
	}

	if err := internalNetlink.ClearVethInterface(containerID[:7], true); err != nil {
		klog.ErrorS(err, "ClearVethInterface failed", "containerID", containerID[:7], "isInternal", true)
		return err
	}
	if err := internalNetlink.ClearVethInterface(containerID[:7], false); err != nil {
		klog.ErrorS(err, "ClearVethInterface failed", "containerID", containerID[:7], "isInternal", false)
		return err
	}

	delete(n.runnigState, containerName)

	klog.InfoS("ClearContainer Done", "ContainerName", containerName)
	return nil
}

func (n *NetworkDaemon) Sync(containerName string, originVlan *int32, internalIPs []string, externalIPs []string, internalCIDR string) error {
	var vlanChanged, internalIPsChanged, externalIPsChanged, internalCIDRChanged bool
	var vlan int

	if internalCIDR == "" {
		klog.Warning("internalCIDR is empty. Nothing to be done")
		return nil
	}

	if originVlan == nil {
		vlan = 0
	} else {
		vlan = int(*originVlan)
	}

	if val, exist := n.runnigState[containerName]; !exist {
		n.runnigState[containerName] = &virtualrouterSpec{}
		if vlan != 0 {
			n.vlanUse[vlan] = append(n.vlanUse[vlan], containerName)
			// n.runnigState[containerName].vlan = int(*vlan)
			vlanChanged = true
		}
		internalIPsChanged = true
		externalIPsChanged = true
		internalCIDRChanged = true
	} else {
		if vlan != n.runnigState[containerName].vlan {
			vlanChanged = true
		}
		if internalCIDR != val.internalCIDR {
			internalCIDRChanged = true
		}
		internalIPsChanged = isIPsChange(val.internalIPs, internalIPs)
		externalIPsChanged = isIPsChange(val.externalIPs, externalIPs)
	}

	// No Change
	if !vlanChanged && !internalIPsChanged && !externalIPsChanged {
		return nil
	}

	// Change
	if err := n.ConnectInterface(containerName, true); err != nil {
		klog.ErrorS(err, "Interface to Container faild", "containerName", containerName)
		return err
	}
	if err := n.ConnectInterface(containerName, false); err != nil {
		klog.ErrorS(err, "Interface to Container faild", "containerName", containerName)
		return err
	}

	if vlanChanged {
		if err := n.AssignVlan(containerName, vlan, n.runnigState[containerName].vlan); err != nil {
			klog.ErrorS(err, "UnssignVlan failed", "containerName", containerName, "vlan", vlan)
			return err
		}
		n.runnigState[containerName].vlan = vlan
	}

	if internalIPsChanged {
		klog.Info(internalIPs)
		if err := n.AssignIPaddress(containerName, internalIPs, true); err != nil {
			klog.ErrorS(err, "AssignIPAddress failed", "containerName", containerName, "IPs", internalIPs)
			return err
		}
		n.runnigState[containerName].internalIPs = internalIPs
	}

	if externalIPsChanged {
		klog.Info(externalIPs)
		if err := n.AssignIPaddress(containerName, externalIPs, false); err != nil {
			klog.ErrorS(err, "AssignVlan failed", "containerName", containerName, "IPs", externalIPs)
			return err
		}
		n.runnigState[containerName].externalIPs = externalIPs
	}

	if internalCIDRChanged {
		klog.Info("InternalCIDR changed")
		if err := n.SetRoute2Container(containerName, internalCIDR); err != nil {
			klog.ErrorS(err, "SetRoute2Container failed", "containerName", containerName, "internalCIDR", internalCIDR)
			return err
		}
		n.runnigState[containerName].internalCIDR = internalCIDR
	}

	return nil
}

func (n *NetworkDaemon) SetRoute2Container(containerName string, cidr string) error {
	var containerID string
	var containerPid int

	containerID = internalCrio.GetContainerIDFromContainerName(containerName, n.crioCfg)
	if containerID == "" {
		klog.Errorf("There is no running container with ContainerName: %s", containerName)
		return fmt.Errorf("no running container found")
	}

	containerPid = internalCrio.GetContainerPid(containerID, n.crioCfg)
	if containerPid <= 0 {
		klog.Errorf("Wrong Pid(%d) value of Container(%s)", containerPid, containerName)
		return fmt.Errorf("internal error")
	}

	if err := internalNetlink.SetRoute2Container(containerPid, containerID[:7], cidr); err != nil {
		klog.ErrorS(err, "Set Routing rule to Container failed", "ContainerName", containerName, "ContainerID", containerID)
		return err
	}
	return nil
}

func (n *NetworkDaemon) AssignVlan(containerName string, newVlan int, oldVlan int) error {
	containerID := internalCrio.GetContainerIDFromContainerName(containerName, n.crioCfg)
	if containerID == "" {
		klog.Errorf("There is no running container with ContainerName: %s", containerName)
		return fmt.Errorf("no running container found")
	}

	if err := internalNetlink.SetVlan("int"+containerID[:7], newVlan, oldVlan, n.netlinkCfg); err != nil {
		klog.ErrorS(err, "SetVlan failed", "vlan", newVlan)
		return err
	}
	return nil
}

// func (n *NetworkDaemon) UnassignVlan(containerName string, vlan int) error {
// 	containerID := internalCrio.GetContainerIDFromContainerName(containerName, n.crioCfg)
// 	if containerID == "" {
// 		klog.Errorf("There is no running container with ContainerName: %s", containerName)
// 		return fmt.Errorf("no running container found")
// 	}

// 	if err := internalNetlink.DeleteVlan(containerID[:7], vlan, n.netlinkCfg); err != nil {
// 		klog.ErrorS(err, "DeleteVlan failed", "vlan", vlan)
// 		return err
// 	}
// 	return nil
// }

func (n *NetworkDaemon) AssignIPaddress(containerName string, cidrs []string, isInternal bool) error {
	var containerID string
	var containerPid int

	containerID = internalCrio.GetContainerIDFromContainerName(containerName, n.crioCfg)
	if containerID == "" {
		klog.Errorf("There is no running container with ContainerName: %s", containerName)
		return fmt.Errorf("no running container found")
	}

	containerPid = internalCrio.GetContainerPid(containerID, n.crioCfg)
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

	containerPid = internalCrio.GetContainerPid(containerID, n.crioCfg)
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

func isIPsChange(a []string, b []string) bool {
	if len(a) != len(b) {
		return true
	}
	sort.Strings(a)
	sort.Strings(b)

	for i := range a {
		if a[i] != b[i] {
			return true
		}
	}

	return false
}

// func (n *NetworkDaemon) ClearInterface(containerName string) error {

// }
