package daemon

import (
	"fmt"

	internalCrio "github.com/tmax-cloud/virtualrouter-controller/internal/daemon/crio"
	internalNetlink "github.com/tmax-cloud/virtualrouter-controller/internal/daemon/netlink"
	v1 "github.com/tmax-cloud/virtualrouter-controller/internal/utils/pkg/apis/networkcontroller/v1"
	"k8s.io/klog/v2"
)

const (
	DEFAULT_MASK_NUMBER                            int    = 200
	DEFAULT_TABLE_NUMBER                           int    = 200
	DEFAULT_VIRTURALROUTER_INTERNAL_INTERFACE_NAME string = "ethint"
	DEFAULT_VIRTURALROUTER_EXTERNAL_INTERFACE_NAME string = "ethext"
)

type NetworkDaemon struct {
	crioCfg          *internalCrio.CrioConfig
	netlinkCfg       *internalNetlink.Config
	runnigState      map[string]*v1.VirtualRouterSpec
	pod2containerMap map[string]*containerDesc
	vlanUse          map[int][]string
}

type containerDesc struct {
	containerName string
	containerID   string
}

// type virtualrouterSpec struct {
// 	vlan         int
// 	internalIPs  []string
// 	externalIPs  []string
// 	internalCIDR string
// }

func NewDaemon(crioCfg *internalCrio.CrioConfig, netlinkCfg *internalNetlink.Config) *NetworkDaemon {
	return &NetworkDaemon{
		crioCfg:          crioCfg,
		netlinkCfg:       netlinkCfg,
		pod2containerMap: make(map[string]*containerDesc),
		runnigState:      make(map[string]*v1.VirtualRouterSpec),
		vlanUse:          make(map[int][]string),
	}
}

func (n *NetworkDaemon) Start(stopSignalCh <-chan struct{}, stopCh chan<- struct{}) error {
	klog.Info("Starting NetworkDaemon")
	klog.Info("Initializing start")

	if err := n.Initialize(); err != nil {
		klog.Error("failed to Initialize")
	} else {
		klog.Info("Initializing done")
	}

	go func() {
		<-stopSignalCh
		// klog.Info("ClearAll start")
		// if err := n.ClearAll(); err != nil {
		// 	klog.Error("failed to Clear")
		// }
		// klog.Info("ClearAll done")
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

func (n *NetworkDaemon) ClearContainer(containerName string, containerID string) error {
	klog.InfoS("ClearContainer Start", "ContainerID", containerID)
	if _, exist := n.runnigState[containerName]; !exist {
		return nil
	}
	if n.runnigState[containerName].VlanNumber != 0 {
		if err := n.AssignVlan(containerName, 0, int(n.runnigState[containerName].VlanNumber)); err != nil {
			return err
		}
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

func (n *NetworkDaemon) AttachingPod(podName string, virtualrouter *v1.VirtualRouter) error {
	var containerName string
	var err error
	if desc, exist := n.pod2containerMap[podName]; exist {
		containerName = desc.containerName
	} else {
		containerID := internalCrio.GetContainerIDFromContainerName(virtualrouter.Name, n.crioCfg)
		if containerID == "" {
			klog.Errorf("There is no running container with ContainerName: %s", containerName)
			return fmt.Errorf("no running container found")
		}
		n.pod2containerMap[podName] = &containerDesc{
			containerName: virtualrouter.Name,
			containerID:   containerID,
		}

		containerName = virtualrouter.Name
	}
	defer func() {
		if err != nil {
			delete(n.pod2containerMap, podName)
		}
	}()

	if _, exist := n.runnigState[containerName]; exist {
		// klog.Warning("Duplicated containerName called. Do nothing")
		return nil
	}

	if err = n.ConnectInterface(containerName, true); err != nil {
		klog.ErrorS(err, "Interface to Container faild", "containerName", containerName)
		return err
	}
	if err = n.ConnectInterface(containerName, false); err != nil {
		klog.ErrorS(err, "Interface to Container faild", "containerName", containerName)
		return err
	}

	if err = n.Sync(containerName, virtualrouter.Spec); err != nil {
		return err
	}

	return nil
}

func (n *NetworkDaemon) DettachingPod(podName string) error {
	var containerID string
	var containerName string
	if desc, exist := n.pod2containerMap[podName]; exist {
		containerName = desc.containerName
		containerID = desc.containerID
	} else {
		return nil
	}

	n.ClearContainer(containerName, containerID)
	delete(n.pod2containerMap, podName)
	return nil
}

func (n *NetworkDaemon) Sync(containerName string, virtualrouterSpec v1.VirtualRouterSpec) error {
	var podExist bool = false
	for _, descs := range n.pod2containerMap {
		if descs.containerName == containerName {
			podExist = true
			break
		}
	}
	if !podExist {
		return nil
	}
	var vlanChanged, internalIPChanged, externalIPChanged, internalNetmaskChanged, externalNetmaskChanged, gatewayIPChanged bool
	var vlan int = int(virtualrouterSpec.VlanNumber)

	if virtualrouterSpecSnapshot, exist := n.runnigState[containerName]; !exist {
		n.runnigState[containerName] = &virtualrouterSpec
		if vlan != 0 {
			n.vlanUse[vlan] = append(n.vlanUse[vlan], containerName)
			vlanChanged = true
		}
		internalIPChanged = true
		externalIPChanged = true
		internalNetmaskChanged = true
		externalNetmaskChanged = true
		gatewayIPChanged = true
		if err := n.SetRouteRule2Container(containerName, DEFAULT_MASK_NUMBER, DEFAULT_TABLE_NUMBER); err != nil {
			return err
		}
	} else {
		if vlan != int(virtualrouterSpecSnapshot.VlanNumber) {
			vlanChanged = true
		}
		if virtualrouterSpec.InternalNetmask != virtualrouterSpecSnapshot.InternalNetmask {
			internalNetmaskChanged = true
		}
		if virtualrouterSpec.ExternalNetmask != virtualrouterSpecSnapshot.ExternalNetmask {
			internalNetmaskChanged = true
		}
		if virtualrouterSpec.InternalIP != virtualrouterSpecSnapshot.InternalIP {
			internalIPChanged = true
		}
		if virtualrouterSpec.ExternalIP != virtualrouterSpecSnapshot.ExternalIP {
			internalIPChanged = true
		}
		if virtualrouterSpec.GatewayIP != virtualrouterSpecSnapshot.GatewayIP {
			gatewayIPChanged = true
		}
	}

	// No Change
	if !vlanChanged && !internalNetmaskChanged && !externalNetmaskChanged && !internalIPChanged && !externalIPChanged && !gatewayIPChanged {
		return nil
	}

	if vlanChanged {
		if err := n.AssignVlan(containerName, vlan, int(n.runnigState[containerName].VlanNumber)); err != nil {
			klog.ErrorS(err, "UnssignVlan failed", "containerName", containerName, "vlan", vlan)
			return err
		}

	}

	if internalIPChanged || internalNetmaskChanged {
		if err := n.AssignIPaddress(containerName, virtualrouterSpec.InternalIP, virtualrouterSpec.InternalNetmask, true); err != nil {
			klog.ErrorS(err, "AssignIPAddress failed", "containerName", containerName, "IPs", virtualrouterSpec.InternalIP)
			return err
		}

	}

	if externalIPChanged || externalNetmaskChanged {
		if err := n.AssignIPaddress(containerName, virtualrouterSpec.ExternalIP, virtualrouterSpec.ExternalNetmask, false); err != nil {
			klog.ErrorS(err, "AssignVlan failed", "containerName", containerName, "IPs", virtualrouterSpec.ExternalIP)
			return err
		}
	}

	if gatewayIPChanged {
		if err := n.SetDefaultRoute2Container(containerName, virtualrouterSpec.GatewayIP); err != nil {
			klog.ErrorS(err, "SetRoute2Container failed", "containerName", containerName, "gatewayIP", virtualrouterSpec.GatewayIP)
			return err
		}
	}

	n.runnigState[containerName] = &virtualrouterSpec
	return nil
}

func (n *NetworkDaemon) SetRouteRule2Container(containerName string, markNumber int, tableNumber int) error {
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

	if err := internalNetlink.SetRouteRule2Container(containerPid, markNumber, tableNumber); err != nil {
		klog.ErrorS(err, "Set Route rule to Container failed", "ContainerName", containerName, "ContainerID", containerID)
		return err
	}
	return nil
}

func (n *NetworkDaemon) SetDefaultRoute2Container(containerName string, gatewayIP string) error {
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

	if err := internalNetlink.SetDefaultRoute2Container(containerPid, gatewayIP, DEFAULT_TABLE_NUMBER); err != nil {
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

func (n *NetworkDaemon) AssignIPaddress(containerName string, ip string, netmask string, isInternal bool) error {
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

	if err := internalNetlink.SetIPaddress2Container(containerPid, ip, netmask, isInternal); err != nil {
		klog.ErrorS(err, "Set Interface to Container failed", "ContainerName", containerName, "ContainerID", containerID)
		return err
	}

	var interfaceName string
	if isInternal {
		interfaceName = DEFAULT_VIRTURALROUTER_INTERNAL_INTERFACE_NAME
	} else {
		interfaceName = DEFAULT_VIRTURALROUTER_EXTERNAL_INTERFACE_NAME
	}
	if err := internalNetlink.SetRoute2Container(containerPid, interfaceName, DEFAULT_TABLE_NUMBER); err != nil {
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
