package main

import (
	crio "github.com/cho4036/virtualrouter-controller/internal/daemon/crio"
)

func main() {

	// daemon.Get_CRICTL_CONFIG()
	crio.TestCriContainerList()
}
