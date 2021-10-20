package main

import (
	"flag"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	daemon "github.com/cho4036/virtualrouter-controller/internal/daemon"
	internalCrio "github.com/cho4036/virtualrouter-controller/internal/daemon/crio"
	internalNetlink "github.com/cho4036/virtualrouter-controller/internal/daemon/netlink"
	clientset "github.com/cho4036/virtualrouter-controller/internal/utils/pkg/generated/clientset/versioned"
	informers "github.com/cho4036/virtualrouter-controller/internal/utils/pkg/generated/informers/externalversions"
	"github.com/cho4036/virtualrouter-controller/internal/utils/pkg/signals"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopSignalCh := signals.SetupSignalHandler()
	stopCh := make(chan struct{})
	cfg, err := rest.InClusterConfig()

	// cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	// if err != nil {
	// 	klog.Fatalf("Error building kubeconfig: %s", err.Error())
	// }

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	exampleClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	exampleInformerFactory := informers.NewSharedInformerFactory(exampleClient, time.Second*30)

	d := daemon.NewDaemon(&internalCrio.CrioConfig{
		RuntimeEndpoint:      "unix:///var/run/crio/crio.sock",
		RuntimeEndpointIsSet: true,
		ImageEndpoint:        "unix:///var/run/crio/crio.sock",
		ImageEndpointIsSet:   true,
		Timeout:              time.Duration(2000000000),
	}, &internalNetlink.Config{
		InternalIPCIDR:        "10.0.0.0/24",
		ExternalIPCIDR:        "192.168.9.0/24",
		InternalInterfaceName: "intif",
		ExternalInterfaceName: "extif",
		InternalBridgeName:    "intbr",
		ExternalBridgeName:    "extbr",
	})
	// if err := d.Initialize(); err != nil {
	// 	klog.ErrorS(err, "Daemon Initialization failed")
	// 	return
	// }

	// defer func(d *daemon.NetworkDaemon) {
	// 	klog.Info("defer called")
	// 	if err := d.ClearAll(); err != nil {
	// 		klog.ErrorS(err, "Daemon ClearAll failed")
	// 	}
	// }(d)

	err = d.Start(stopSignalCh, stopCh)
	if err != nil {
		klog.Errorf("Error running network daemon: %s", err.Error())
	}

	controller := daemon.NewController(kubeClient, exampleClient, d,
		exampleInformerFactory.Tmax().V1().VirtualRouters())

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(stopCh)
	exampleInformerFactory.Start(stopCh)

	if err = controller.Run(1, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}

}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
