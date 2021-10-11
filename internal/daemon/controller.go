/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package daemon

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	clientset "github.com/cho4036/virtualrouter-controller/internal/utils/pkg/generated/clientset/versioned"
	samplescheme "github.com/cho4036/virtualrouter-controller/internal/utils/pkg/generated/clientset/versioned/scheme"
	informers "github.com/cho4036/virtualrouter-controller/internal/utils/pkg/generated/informers/externalversions/networkcontroller/v1"
	listers "github.com/cho4036/virtualrouter-controller/internal/utils/pkg/generated/listers/networkcontroller/v1"
)

const controllerAgentName = "virtual-router"

// var virtualRouterNamespace = "virtualrouter"

// const (
// 	serviceAccountName     = "virtualrouter-sa"
// 	clusterRoleName        = "virtualrouter-role"
// 	clusterRoleBindingName = "virtualrouter-rb"
// )

const (
	// SuccessSynced is used as part of the Event 'reason' when a VirtualRouter is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a VirtualRouter fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by VirtualRouter"
	// MessageResourceSynced is the message used for an Event fired when a VirtualRouter
	// is synced successfully
	MessageResourceSynced = "VirtualRouter synced successfully"
)

// Controller is the controller implementation for VirtualRouter resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// sampleclientset is a clientset for our own API group
	sampleclientset clientset.Interface

	networkDaemon *NetworkDaemon

	// podLister            corelisters.PodLister
	// podSynced            cache.InformerSynced
	// deploymentsLister    appslisters.DeploymentLister
	// deploymentsSynced    cache.InformerSynced
	virtualRoutersLister listers.VirtualRouterLister
	virtualRoutersSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	sampleclientset clientset.Interface,
	daemon *NetworkDaemon,
	// deploymentInformer appsinformers.DeploymentInformer,
	// podInformer coreinformers.PodInformer,
	virtualRouterInformer informers.VirtualRouterInformer) *Controller {

	// Create event broadcaster
	// Add virtual-router types to the default Kubernetes Scheme so Events can be
	// logged for virtual-router types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:   kubeclientset,
		sampleclientset: sampleclientset,
		networkDaemon:   daemon,
		// deploymentsLister:    deploymentInformer.Lister(),
		// deploymentsSynced:    deploymentInformer.Informer().HasSynced,
		// podLister:            podInformer.Lister(),
		// podSynced:            podInformer.Informer().HasSynced,
		virtualRoutersLister: virtualRouterInformer.Lister(),
		virtualRoutersSynced: virtualRouterInformer.Informer().HasSynced,
		workqueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "VirtualRouters"),
		recorder:             recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when VirtualRouter resources change
	virtualRouterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueVirtualRouter,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueVirtualRouter(new)
		},
		DeleteFunc: controller.enqueueVirtualRouter,
	})

	// podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
	// 	AddFunc: controller.enqueuePod,
	// 	UpdateFunc: func(old, new interface{}) {
	// 		newPod := new.(*corev1.Pod)
	// 		oldPod := old.(*corev1.Pod)
	// 		if newPod.ResourceVersion == oldPod.ResourceVersion {
	// 			// Periodic resync will send update events for all known Deployments.
	// 			// Two different versions of the same Deployment will always have different RVs.
	// 			return
	// 		}
	// 		controller.enqueuePod(new)
	// 	},
	// 	DeleteFunc: controller.enqueuePod,
	// })

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting VirtualRouter controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	// if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.virtualRoutersSynced); !ok {
	if ok := cache.WaitForCacheSync(stopCh, c.virtualRoutersSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process VirtualRouter resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// VirtualRouter resource to be synced.
		// if err := c.syncHandler(key); err != nil {
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the VirtualRouter resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	klog.Info(key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	virtualRouter, err := c.virtualRoutersLister.VirtualRouters(namespace).Get(name)
	if err != nil {
		// The VirtualRouter resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			c.networkDaemon.ClearContainer(name)
			utilruntime.HandleError(fmt.Errorf("virtualRouter '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}
	if err := c.networkDaemon.Sync(name, virtualRouter.Spec.VlanNumber, virtualRouter.Spec.InternalIPs, virtualRouter.Spec.ExternalIPs); err != nil {
		klog.ErrorS(err, "Sync failed")
		return err
	}
	// dpName := virtualRouter.ObjectMeta.GetName()
	// dpNamespace := virtualRouter.ObjectMeta.GetNamespace()
	// deployment, err := c.deploymentsLister.Deployments(dpNamespace).Get(dpName)
	// if err != nil {
	// 	if errors.IsNotFound(err) {
	// 		klog.Info("NotFound Deploy start")
	// 	} else{
	// 		return err
	// 	}
	// }
	// deployment.Spec.Template.Spec.Containers

	// pod, err := c.podLister.Pods(namespace).Get()
	// case podKey:

	// Convert the namespace/name string into a distinct namespace and name

	// Get the VirtualRouter resource with this namespace/name

	// if err != nil {
	// 	// The VirtualRouter resource may no longer exist, in which case we stop
	// 	// processing.
	// 	if errors.IsNotFound(err) {
	// 		utilruntime.HandleError(fmt.Errorf("virtualRouter '%s' in work queue no longer exists", key))
	// 		return nil
	// 	}

	// 	return err
	// }

	// deploymentName := virtualRouter.Spec.DeploymentName
	// if deploymentName == "" {
	// 	// We choose to absorb the error here as the worker would requeue the
	// 	// resource otherwise. Instead, the next time the resource is updated
	// 	// the resource will be queued again.
	// 	utilruntime.HandleError(fmt.Errorf("%s: deployment name must be specified", key))
	// 	return nil
	// }

	// // Get the deployment with the name specified in VirtualRouter.spec
	// deployment, err := c.deploymentsLister.Deployments(virtualRouter.Namespace).Get(deploymentName)
	// // If the resource doesn't exist, we'll create it
	// if errors.IsNotFound(err) {
	// 	klog.Info("NotFound Deploy start")
	// 	deployment, err = c.kubeclientset.AppsV1().Deployments(virtualRouter.Namespace).Create(context.TODO(), newDeployment(virtualRouter), metav1.CreateOptions{})
	// }

	// // If an error occurs during Get/Create, we'll requeue the item so we can
	// // attempt processing again later. This could have been caused by a
	// // temporary network failure, or any other transient reason.
	// if err != nil {
	// 	return err
	// }

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	// if err != nil {
	// 	return err
	// }

	// // Finally, we update the status block of the VirtualRouter resource to reflect the
	// // current state of the world
	// err = c.updateVirtualRouterStatus(virtualRouter, deployment)
	// if err != nil {
	// 	return err
	// }

	// c.recorder.Event(virtualRouter, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// enqueueVirtualRouter takes a VirtualRouter resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than VirtualRouter.
func (c *Controller) enqueueVirtualRouter(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}
