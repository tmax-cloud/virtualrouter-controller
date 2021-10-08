package daemon_test

import (
	"fmt"
	"testing"

	daemon "github.com/cho4036/virtualrouter-controller/internal/daemon"
	"github.com/cho4036/virtualrouter-controller/internal/daemon/crio"
	"github.com/cho4036/virtualrouter-controller/internal/daemon/netlink"
)

var _ daemon.NetworkDaemon

func TestDaemonInitialize(t *testing.T) {
	d := daemon.NewDaemon(&crio.CrioConfig{}, &netlink.Config{
		InternalIPCIDR:        "10.0.0.0/24",
		ExternalIPCIDR:        "192.168.9.0/24",
		InternalInterfaceName: "intif",
		ExternalInterfaceName: "extif",
		InternalBridgeName:    "intbr",
		ExternalBridgeName:    "extbr",
	})
	if err := d.Initialize(); err != nil {
		fmt.Errorf("Error: %+v", err)
	}
}

func TestDaemonClear(t *testing.T) {
	d := daemon.NewDaemon(&crio.CrioConfig{}, &netlink.Config{
		InternalIPCIDR:        "10.0.0.0/24",
		ExternalIPCIDR:        "192.168.9.0/24",
		InternalInterfaceName: "intif",
		ExternalInterfaceName: "extif",
		InternalBridgeName:    "intbr",
		ExternalBridgeName:    "extbr",
	})
	if err := d.ClearAll(); err != nil {
		fmt.Errorf("Error: %+v", err)
	}
}
