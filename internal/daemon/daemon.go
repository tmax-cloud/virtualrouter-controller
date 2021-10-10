package daemon

import (
	"fmt"

	internalCrio "github.com/cho4036/virtualrouter-controller/internal/daemon/crio"
	internalNetlink "github.com/cho4036/virtualrouter-controller/internal/daemon/netlink"
	"k8s.io/klog/v2"
)

type NetworkDaemon struct {
	crioCfg    *internalCrio.CrioConfig
	netlinkCfg *internalNetlink.Config
}

func NewDaemon(crioCfg *internalCrio.CrioConfig, netlinkCfg *internalNetlink.Config) *NetworkDaemon {
	return &NetworkDaemon{
		crioCfg:    crioCfg,
		netlinkCfg: netlinkCfg,
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

func (n *NetworkDaemon) ConnectInterface(containerName string, cidr string, isInternal bool) error {
	var containerID string
	var containerPid int

	containerID = internalCrio.GetContainerIDFromContainerName(containerName, n.crioCfg)
	if containerID == "" {
		klog.Errorf("There is no container with ContainerName: %s", containerName)
		return fmt.Errorf("not found")
	}

	containerPid = internalCrio.GetContainerPid(containerID)
	if containerPid <= 0 {
		klog.Errorf("Wrong Pid(%d) value of Container(%s)", containerPid, containerName)
		return fmt.Errorf("internal error")
	}

	if err := internalNetlink.SetInterface2Container(containerPid, containerID[:7], cidr, isInternal, n.netlinkCfg); err != nil {
		klog.ErrorS(err, "Set Interface to Container failed", "ContainerName", containerName, "ContainerID", containerID)
		return err
	}
	return nil
}
