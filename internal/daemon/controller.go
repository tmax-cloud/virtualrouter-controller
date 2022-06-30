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
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	clientset "github.com/tmax-cloud/virtualrouter-controller/internal/utils/pkg/generated/clientset/versioned"
	samplescheme "github.com/tmax-cloud/virtualrouter-controller/internal/utils/pkg/generated/clientset/versioned/scheme"
	informers "github.com/tmax-cloud/virtualrouter-controller/internal/utils/pkg/generated/informers/externalversions/networkcontroller/v1"
	listers "github.com/tmax-cloud/virtualrouter-controller/internal/utils/pkg/generated/listers/networkcontroller/v1"
	"github.com/tmax-cloud/virtualrouter-controller/internal/virtualroutermanager"
	v1Pod "k8s.io/kubernetes/pkg/api/v1/pod"
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

type podKey string
type virtualrouterKey string

// Controller is the controller implementation for VirtualRouter resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// sampleclientset is a clientset for our own API group
	sampleclientset clientset.Interface

	networkDaemon *NetworkDaemon

	podLister corelisters.PodLister
	podSynced cache.InformerSynced

	virtualRoutersLister listers.VirtualRouterLister
	virtualRoutersSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface

	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	sampleclientset clientset.Interface,
	daemon *NetworkDaemon,
	podInformer coreinformers.PodInformer,
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
		kubeclientset:        kubeclientset,
		sampleclientset:      sampleclientset,
		networkDaemon:        daemon,
		podLister:            podInformer.Lister(),
		podSynced:            podInformer.Informer().HasSynced,
		virtualRoutersLister: virtualRouterInformer.Lister(),
		virtualRoutersSynced: virtualRouterInformer.Informer().HasSynced,
		workqueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "VirtualRouters"),
		recorder:             recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when VirtualRouter resources change
	virtualRouterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		// AddFunc: controller.enqueueVirtualRouter,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueVirtualRouter(new)
		},
		// DeleteFunc: controller.enqueueVirtualRouter,
	})

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueuePod,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueuePod(new)
		},
		// DeleteFunc: controller.enqueuePod,
	})

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

	if err := c.syncHandler(obj); err != nil {
		c.workqueue.AddRateLimited(obj)
		var objName string
		switch key := obj.(type) {
		case podKey:
			objName = (string)(podKey(key))
		case virtualrouterKey:
			objName = (string)(virtualrouterKey(key))
		}
		klog.Errorf("error syncing '%s': %s, requeuing", objName, err.Error())

	} else {
		c.workqueue.Forget(obj)
	}
	c.workqueue.Done(obj)

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the VirtualRouter resource
// with the current status of the resource.
func (c *Controller) syncHandler(obj interface{}) error {
	switch key := obj.(type) {
	case podKey:
		namespace, name, err := cache.SplitMetaNamespaceKey(string(key))
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
			return nil
		}

		virtualRouterPod, err := c.podLister.Pods(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				klog.Info("Pod deletion called")
				// c.networkDaemon.DettachingPod(name)
				utilruntime.HandleError(fmt.Errorf("virtualRouter '%s' in work queue no longer exists", key))
				return nil
			}
			return err
		}
		if !virtualRouterPod.DeletionTimestamp.IsZero() {
			if err := c.networkDaemon.DettachingPod(name); err != nil {
				return err
			}
			if err := c.deleteFinalizer(name, virtualRouterPod); err != nil {
				return err
			}
			return nil
		}
		if !v1Pod.IsPodReady(virtualRouterPod) {
			return nil
		}

		crName := virtualRouterPod.GetAnnotations()["customresourceName"]
		crNS := virtualRouterPod.GetAnnotations()["customresourceNamespace"]
		if crName == "" || crNS == "" {
			return fmt.Errorf("pod's annotations are missing")
		}
		virtualRouterCR, err := c.virtualRoutersLister.VirtualRouters(crNS).Get(crName)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}

		if err := c.networkDaemon.AttachingPod(name, virtualRouterCR); err != nil {
			klog.ErrorS(err, "Sync failed")
			return err
		}

		klog.Infof("Successfully synced '%s'", string(key))

	case virtualrouterKey:
		namespace, name, err := cache.SplitMetaNamespaceKey(string(key))
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
			return nil
		}

		virtualRouterCR, err := c.virtualRoutersLister.VirtualRouters(namespace).Get(name)
		if err != nil {
			// The VirtualRouter resource may no longer exist, in which case we stop
			// processing.
			if errors.IsNotFound(err) {
				utilruntime.HandleError(fmt.Errorf("virtualRouter '%s' in work queue no longer exists", key))
				return nil
			}
			return err
		}

		if err := c.networkDaemon.Sync(name, virtualRouterCR.Spec); err != nil {
			klog.ErrorS(err, "Sync failed")
			return err
		}

		klog.Infof("Successfully synced '%s'", string(key))
	}
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
	c.workqueue.Add(virtualrouterKey(key))
}

func (c *Controller) enqueuePod(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(podKey(key))
}

func (c *Controller) deleteFinalizer(podName string, virtualrouterPod *corev1.Pod) error {
	if containsString(virtualrouterPod.ObjectMeta.Finalizers, virtualroutermanager.VIRTUALROUTER_DAEMON_FINALIZER) {
		virtualrouterPodCopy := virtualrouterPod.DeepCopy()
		virtualrouterPodCopy.ObjectMeta.Finalizers = removeString(virtualrouterPodCopy.ObjectMeta.Finalizers, virtualroutermanager.VIRTUALROUTER_DAEMON_FINALIZER)
		_, err := c.kubeclientset.CoreV1().Pods(virtualrouterPodCopy.Namespace).Update(context.TODO(), virtualrouterPodCopy, v1.UpdateOptions{})
		if err != nil {
			klog.Errorln("Deleteing finalizer is failed in some reason")
			return err
		}
	}
	return nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
