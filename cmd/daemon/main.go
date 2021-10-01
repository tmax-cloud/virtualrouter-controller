package main

import (
	daemon "github.com/cho4036/virtualrouter-controller/internal/daemon"
)

func main() {

	// daemon.Get_CRICTL_CONFIG()
	daemon.TestCriContainerList()
}
