package daemon_test

import (
	"fmt"
	"testing"
	"time"

	daemon "github.com/tmax-cloud/virtualrouter-controller/internal/daemon"
	"github.com/tmax-cloud/virtualrouter-controller/internal/daemon/crio"
	"github.com/tmax-cloud/virtualrouter-controller/internal/daemon/netlink"
)

var _ daemon.NetworkDaemon

func TestDaemonInitialize(t *testing.T) {
	d := daemon.NewDaemon(
		&crio.CrioConfig{
			RuntimeEndpoint:      "unix:///var/run/crio/crio.sock",
			RuntimeEndpointIsSet: true,
			ImageEndpoint:        "unix:///var/run/crio/crio.sock",
			ImageEndpointIsSet:   true,
			Timeout:              time.Duration(2000000000),
		}, &netlink.Config{
			InternalIPCIDR:           "10.0.0.0/24",
			ExternalIPCIDR:           "192.168.9.0/24",
			NewInternalInterfaceName: "intif",
			NewExternalInterfaceName: "extif",
			InternalBridgeName:       "intbr",
			ExternalBridgeName:       "extbr",
		})
	if err := d.Initialize(); err != nil {
		fmt.Errorf("Error: %+v", err)
	}

	// crio.GetContainerIDFromContainerName("", d.)

	fmt.Println("Initailize done")

	if err := d.ConnectInterface("virtualrouter1", true); err != nil {
		fmt.Errorf("Error: %+v", err)
	}

	fmt.Println("ConnectInterface done")

	if err := d.ClearAll(); err != nil {
		fmt.Errorf("Error: %+v", err)
	}

	fmt.Println("Clear done")
}
