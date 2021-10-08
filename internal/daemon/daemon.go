package daemon

import (
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
