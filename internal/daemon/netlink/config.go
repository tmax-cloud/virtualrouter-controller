package netlink

type Config struct {
	InternalBridgeName string
	ExternalBridgeName string

	OriginInternalInterfaceName string
	OriginExternalInterfaceName string

	NewInternalInterfaceName string
	NewExternalInterfaceName string

	InternalIPCIDR string
	ExternalIPCIDR string

	InternalIP      string
	InternalNetmask string

	ExternalIP      string
	ExternalNetmaks string

	GatewayIP string
}
