package netlink_test

import (
	"testing"

	internalNetlink "github.com/cho4036/virtualrouter-controller/internal/daemon/netlink"
)

func TestDD(t *testing.T) {
	internalNetlink.DD()
}

func TestPBR(t *testing.T) {
	internalNetlink.PBR()
}
func TestDaemon(t *testing.T) {
	// var rootNetlinkHandle *remoteNetlink.Handle
	// var err error
	// // var targetPid int

	// if rootNetlinkHandle, err = internalNetlink.GetRootNetlinkHandle(); err != nil {
	// 	fmt.Println(err)
	// }

	// cfg := &internalNetlink.Config{
	// 	InternalBridgeName: "testInt",
	// 	ExternalBridgeName: "testExt",
	// }
	// if err := internalNetlink.Initialize(cfg); err != nil {
	// 	fmt.Println(err)
	// }

	// if _, _, err := internalNetlink.SetVethInterface(rootNetlinkHandle, "testVeth"); err != nil {
	// 	fmt.Println(err)
	// }

	// var targetNetlinkHandle *remoteNetlink.Handle
	// var targetNsHandle netns.NsHandle
	// if targetNsHandle, err = internalNetlink.GetNsHandle(internalNetlink.CrioType(targetPid)); err != nil {
	// 	fmt.Println(err)
	// }

	// if targetNetlinkHandle, err = internalNetlink.GetTargetNetlinkHandle(targetNsHandle); err != nil {
	// 	fmt.Println(err)
	// }

	// var veth
	// if err := internalNetlink.SetVethToContainer(rootNetlinkHandle, targetNetlinkHandle,  )

	// if err := internalNetlink.Clear(cfg); err != nil {
	// 	fmt.Println(err)
	// }
}
