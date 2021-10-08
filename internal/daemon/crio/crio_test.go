package crio_test

import (
	"testing"

	daemon "github.com/cho4036/virtualrouter-controller/internal/daemon/crio"
)

func TestDaemon(t *testing.T) {
	// daemon.GetContainerPid("")
	// daemon.Get_CRICTL_CONFIG()
	// daemon.TestCriContainerList()
	s := daemon.Preinit()
	daemon.GetContainerPid(s)

	// daemon.NetDial()
}
