package crio_test

import (
	"testing"
	"time"

	internalCrio "github.com/cho4036/virtualrouter-controller/internal/daemon/crio"
)

func TestDaemon(t *testing.T) {
	// daemon.GetContainerPid("")
	// daemon.Get_CRICTL_CONFIG()
	// daemon.TestCriContainerList()
	// s := daemon.Preinit()
	// daemon.GetContainerPid(s)

	// daemon.NetDial()
	internalCrio.RuntimeServiceTestfunc(&internalCrio.CrioConfig{
		RuntimeEndpoint:      "unix:///var/run/crio/crio.sock",
		RuntimeEndpointIsSet: true,
		ImageEndpoint:        "unix:///var/run/crio/crio.sock",
		ImageEndpointIsSet:   true,
		Timeout:              time.Duration(2000000000),
	})
}
