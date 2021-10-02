package daemon

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	cri "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	remote "k8s.io/kubernetes/pkg/kubelet/cri/remote"

	"k8s.io/klog/v2"
)

const (
	unixProtocol = "unix"
)

var (
	crio_config_map           = make(map[string]string)
	CRIO_CONFIG_DIR           = "/etc/crictl.yaml"
	DEFAULT_RUNTIME_ENDPOINTS = []string{"unix:///var/run/crio/crio.sock"}
	DEFAULT_IMAGE_ENDPOINTS   = []string{"unix:///var/run/crio/crio.sock"}

	// RuntimeEndpoint is CRI server runtime endpoint
	RuntimeEndpoint string
	// RuntimeEndpointIsSet is true when RuntimeEndpoint is configured
	RuntimeEndpointIsSet bool
	// ImageEndpoint is CRI server image endpoint, default same as runtime endpoint
	ImageEndpoint string
	// ImageEndpointIsSet is true when ImageEndpoint is configured
	ImageEndpointIsSet bool
	// Timeout  of connecting to server (default: 10s)
	Timeout time.Duration
	// Debug enable debug output
	Debug bool
	// PullImageOnCreate enables pulling image on create requests
	PullImageOnCreate bool
	// DisablePullOnRun disable pulling image on run requests
	DisablePullOnRun bool
)

func NetDial() {
	_, err := net.Dial("unix", "/var/run/crio/crio.sock")
	if err != nil {
		fmt.Println(err)
	}

}

//// kubelet code cloned
func Preinit() {
	// Get_CRICTL_CONFIG()
	var remoteRuntimeService cri.RuntimeService
	var remoteImageService cri.ImageManagerService
	var err error

	// fmt.Println(DEFAULT_RUNTIME_ENDPOINTS)
	// fmt.Println(Timeout)
	Timeout, err = time.ParseDuration("5s")
	if err != nil {
		fmt.Println(err)
	}
	// fmt.Println(Timeout)
	if remoteRuntimeService, err = remote.NewRemoteRuntimeService(DEFAULT_RUNTIME_ENDPOINTS[0], Timeout); err != nil {
		return
	}
	// if remoteRuntimeService, err = remote.NewRemoteRuntimeService(strings.Split(RuntimeEndpoint, "unix://")[1], Timeout); err != nil {
	// 	return
	// }
	l, err := remoteRuntimeService.ListContainers(nil)
	// fmt.Println(l)

	for _, container := range l {
		fmt.Println(container.GetLabels()["io.kubernetes.pod.name"])
	}
	if remoteImageService, err = remote.NewRemoteImageService(DEFAULT_IMAGE_ENDPOINTS[0], Timeout); err != nil {
		return
	}

	remoteImageService.ListImages(&runtimeapi.ImageFilter{})
}

/////

func Get_CRICTL_CONFIG() {
	file, err := os.Open(CRIO_CONFIG_DIR)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		line, isPrefix, err := reader.ReadLine()
		if isPrefix || err != nil {
			break
		}
		str := string(line)
		strs := strings.Split(str, ": ")
		crio_config_map[strs[0]] = strs[1]
	}

	if val, ok := crio_config_map["runtime-endpoint"]; ok {
		RuntimeEndpoint = val
		RuntimeEndpointIsSet = true
	}

	if val, ok := crio_config_map["image-endpoint"]; ok {
		ImageEndpoint = val
		ImageEndpointIsSet = true
	}

	if val, ok := crio_config_map["timeout"]; ok {
		Timeout, err = time.ParseDuration(val)
		if err != nil {
			panic(err)
		}
	}

	if val, ok := crio_config_map["debug"]; ok {
		Debug, err = strconv.ParseBool(val)
		if err != nil {
			panic(err)
		}
	}

	if val, ok := crio_config_map["pull-image-on-create"]; ok {
		PullImageOnCreate, err = strconv.ParseBool(val)
		if err != nil {
			panic(err)
		}
	}
	if val, ok := crio_config_map["disable-pull-on-run"]; ok {
		DisablePullOnRun, err = strconv.ParseBool(val)
		if err != nil {
			panic(err)
		}
	}

}

func TestCriContainerList() {
	Get_CRICTL_CONFIG()
	runtimeClient, runtimeConn, err := getRuntimeClient()
	if err != nil {
		fmt.Println(err)
		return
		// return err
	}
	defer closeConnection(runtimeConn)
	fmt.Println("RuntimeClinet done")
	imageClient, imageConn, err := getImageClient()
	if err != nil {
		fmt.Println(err)
		return
		// return err
	}
	defer closeConnection(imageConn)
	fmt.Println("ImageClient done")
	opts := listOptions{}
	if err = ListContainers(runtimeClient, imageClient, opts); err != nil {
		// return errors.Wrap(err, "listing containers")
		return
	}
}

func closeConnection(conn *grpc.ClientConn) error {
	if conn == nil {
		return nil
	}
	return conn.Close()
}

// code clone from https://github.com/kubernetes/kubernetes/blob/v1.22.2/pkg/kubelet/util/util_unix.go#L82
// due to broken package dependency
//// Start
func GetAddressAndDialer(endpoint string) (string, func(ctx context.Context, addr string) (net.Conn, error), error) {
	protocol, addr, err := parseEndpointWithFallbackProtocol(endpoint, unixProtocol)
	if err != nil {
		return "", nil, err
	}
	if protocol != unixProtocol {
		return "", nil, fmt.Errorf("only support unix socket endpoint")
	}

	return addr, dial, nil
}

func dial(ctx context.Context, addr string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, unixProtocol, addr)
}

func parseEndpointWithFallbackProtocol(endpoint string, fallbackProtocol string) (protocol string, addr string, err error) {
	if protocol, addr, err = parseEndpoint(endpoint); err != nil && protocol == "" {
		fallbackEndpoint := fallbackProtocol + "://" + endpoint
		protocol, addr, err = parseEndpoint(fallbackEndpoint)
		if err == nil {
			klog.InfoS("Using this endpoint is deprecated, please consider using full URL format", "endpoint", endpoint, "URL", fallbackEndpoint)
		}
	}
	return
}

func parseEndpoint(endpoint string) (string, string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", err
	}

	switch u.Scheme {
	case "tcp":
		return "tcp", u.Host, nil

	case "unix":
		return "unix", u.Path, nil

	case "":
		return "", "", fmt.Errorf("using %q as endpoint is deprecated, please consider using full url format", endpoint)

	default:
		return u.Scheme, "", fmt.Errorf("protocol %q not supported", u.Scheme)
	}
}

//// End

func getConnection(endPoints []string) (*grpc.ClientConn, error) {
	if endPoints == nil || len(endPoints) == 0 {
		return nil, fmt.Errorf("endpoint is not set")
	}
	endPointsLen := len(endPoints)
	var conn *grpc.ClientConn
	for indx, endPoint := range endPoints {
		klog.Info("connect using endpoint '%s' with '%s' timeout", endPoint, Timeout)
		addr, dialer, err := GetAddressAndDialer(endPoint)
		if err != nil {
			if indx == endPointsLen-1 {
				return nil, err
			}
			klog.Error(err)
			continue
		}

		// addr = "unix:///var/run/crio/crio.sock"
		klog.InfoS("getAddressAndDialer done", "addr", addr, "dialer", dialer, "Timeout", Timeout, "isSet", RuntimeEndpointIsSet)
		// conn, err = grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(Timeout), grpc.WithContextDialer(dialer))
		conn, err = grpc.Dial(addr, grpc.WithInsecure(), grpc.WithContextDialer(dialer))
		klog.Info("grpc dial done")
		if err != nil {
			errMsg := errors.Wrapf(err, "connect endpoint '%s', make sure you are running as root and the endpoint has been started", endPoint)
			if indx == endPointsLen-1 {
				return nil, errMsg
			}
			klog.Error(errMsg)
		} else {
			klog.Info("connected successfully using endpoint: %s", endPoint)
			break
		}
	}
	return conn, nil
}

func getRuntimeClientConnection() (*grpc.ClientConn, error) {
	if RuntimeEndpointIsSet && RuntimeEndpoint == "" {
		return nil, fmt.Errorf("--runtime-endpoint is not set")
	}
	klog.Info("get runtime connection")
	// If no EP set then use the default endpoint types
	if !RuntimeEndpointIsSet {
		klog.Warningf("runtime connect using default endpoints: %v. "+
			"As the default settings are now deprecated, you should set the "+
			"endpoint instead.", DEFAULT_RUNTIME_ENDPOINTS)
		klog.Warningf("Note that performance maybe affected as each default " +
			"connection attempt takes n-seconds to complete before timing out " +
			"and going to the next in sequence.")
		return getConnection(DEFAULT_RUNTIME_ENDPOINTS)
	}
	return getConnection([]string{RuntimeEndpoint})
}

func getImageClientConnection() (*grpc.ClientConn, error) {
	if ImageEndpoint == "" {
		if RuntimeEndpointIsSet && RuntimeEndpoint == "" {
			return nil, fmt.Errorf("--image-endpoint is not set")
		}
		ImageEndpoint = RuntimeEndpoint
		ImageEndpointIsSet = RuntimeEndpointIsSet
	}
	klog.Info("get image connection")
	// If no EP set then use the default endpoint types
	if !ImageEndpointIsSet {
		klog.Warningf("image connect using default endpoints: %v. "+
			"As the default settings are now deprecated, you should set the "+
			"endpoint instead.", DEFAULT_IMAGE_ENDPOINTS)
		klog.Warningf("Note that performance maybe affected as each default " +
			"connection attempt takes n-seconds to complete before timing out " +
			"and going to the next in sequence.")
		return getConnection(DEFAULT_IMAGE_ENDPOINTS)
	}
	return getConnection([]string{ImageEndpoint})
}

func getRuntimeClient() (runtimeapi.RuntimeServiceClient, *grpc.ClientConn, error) {
	// Set up a connection to the server.
	conn, err := getRuntimeClientConnection()
	if err != nil {
		return nil, nil, errors.Wrap(err, "connect")
	}
	runtimeClient := runtimeapi.NewRuntimeServiceClient(conn)
	return runtimeClient, conn, nil
}

func getImageClient() (runtimeapi.ImageServiceClient, *grpc.ClientConn, error) {
	// Set up a connection to the server.
	conn, err := getImageClientConnection()
	if err != nil {
		return nil, nil, errors.Wrap(err, "connect")
	}
	imageClient := runtimeapi.NewImageServiceClient(conn)
	return imageClient, conn, nil
}

//////////

type listOptions struct {
	// id of container or sandbox
	id string
	// podID of container
	podID string
	// Regular expression pattern to match pod or container
	nameRegexp string
	// Regular expression pattern to match the pod namespace
	podNamespaceRegexp string
	// state of the sandbox
	state string
	// show verbose info for the sandbox
	verbose bool
	// labels are selectors for the sandbox
	labels map[string]string
	// quiet is for listing just container/sandbox/image IDs
	quiet bool
	// output format
	output string
	// all containers
	all bool
	// latest container
	latest bool
	// last n containers
	last int
	// out with truncating the id
	noTrunc bool
	// image used by the container
	image string
}

func matchesRegex(pattern, target string) bool {
	if pattern == "" {
		return true
	}
	matched, err := regexp.MatchString(pattern, target)
	if err != nil {
		// Assume it's not a match if an error occurs.
		return false
	}
	return matched
}

// type containerByCreated []*runtimeapi.Container

func getContainersList(containersList []*runtimeapi.Container, opts listOptions) []*runtimeapi.Container {
	filtered := []*runtimeapi.Container{}
	for _, c := range containersList {
		// Filter by pod name/namespace regular expressions.
		if matchesRegex(opts.nameRegexp, c.Metadata.Name) {
			filtered = append(filtered, c)
		}
	}

	n := len(filtered)
	if opts.latest {
		n = 1
	}
	if opts.last > 0 {
		n = opts.last
	}
	n = func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}(n, len(filtered))

	return filtered[:n]
}

func ListContainers(runtimeClient runtimeapi.RuntimeServiceClient, imageClient runtimeapi.ImageServiceClient, opts listOptions) error {
	filter := &runtimeapi.ContainerFilter{}
	if opts.id != "" {
		filter.Id = opts.id
	}
	if opts.podID != "" {
		filter.PodSandboxId = opts.podID
	}
	st := &runtimeapi.ContainerStateValue{}
	if !opts.all && opts.state == "" {
		st.State = runtimeapi.ContainerState_CONTAINER_RUNNING
		filter.State = st
	}
	if opts.state != "" {
		st.State = runtimeapi.ContainerState_CONTAINER_UNKNOWN
		switch strings.ToLower(opts.state) {
		case "created":
			st.State = runtimeapi.ContainerState_CONTAINER_CREATED
			filter.State = st
		case "running":
			st.State = runtimeapi.ContainerState_CONTAINER_RUNNING
			filter.State = st
		case "exited":
			st.State = runtimeapi.ContainerState_CONTAINER_EXITED
			filter.State = st
		case "unknown":
			st.State = runtimeapi.ContainerState_CONTAINER_UNKNOWN
			filter.State = st
		default:
			klog.Error("--state should be one of created, running, exited or unknown")
		}
	}
	if opts.latest || opts.last > 0 {
		// Do not filter by state if latest/last is specified.
		filter.State = nil
	}
	if opts.labels != nil {
		filter.LabelSelector = opts.labels
	}
	request := &runtimeapi.ListContainersRequest{
		Filter: filter,
	}
	klog.Info("ListContainerRequest: %v", request)

	r, err := runtimeClient.ListContainers(context.Background(), request)
	klog.Info("ListContainerResponse: %v", r)

	if err != nil {
		fmt.Println(err)
		return err
	}
	r.Containers = getContainersList(r.GetContainers(), opts)
	switch opts.output {
	// case "json":
	// 	return outputProtobufObjAsJSON(r)
	// case "yaml":
	// 	return outputProtobufObjAsYAML(r)
	case "table":
	// continue; output will be generated after the switch block ends.
	default:
		return fmt.Errorf("unsupported output format %q", opts.output)
	}
	fmt.Println("ddd")
	fmt.Println(len(r.Containers))
	for _, c := range r.Containers {
		fmt.Println(c)
	}

	return nil
}

// type Service interface {
// 	v1.RuntimeServiceServer
// 	v1.ImageServiceServer
// }

// type service struct {
// 	server *crioServer.Server
// }

// func Register(grpcServer *grpc.Server, server *crioServer.Server) {
// 	crioServer.New(context.TODO(), server.Config())
// 	s := &service{server}
// 	v1.RegisterRuntimeServiceServer(grpcServer, s)
// 	v1.RegisterImageServiceServer(grpcServer, s)
// }
// func CrioClient() {
// 	var opts []grpc.ServerOption
// 	grpc.NewServer(opts...)
// 	c := new(v1.Container)
// 	c.Descriptor()
// }
