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

package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

	nicolev1alpha1 "k8s.io/k8s-controller/pkg/apis/nicolecontroller/v1alpha1"
	clientset "k8s.io/k8s-controller/pkg/client_icecream/clientset/versioned"
	samplescheme "k8s.io/k8s-controller/pkg/client_icecream/clientset/versioned/scheme"
	informers "k8s.io/k8s-controller/pkg/client_icecream/informers/externalversions/nicolecontroller/v1alpha1"
	listers "k8s.io/k8s-controller/pkg/client_icecream/listers/nicolecontroller/v1alpha1"
)

const icecreamControllerAgentName = "nicole-controller"

const (
	// IcecreamSuccessSynced is used as part of the Event 'reason' when a Icecream is synced
	IcecreamSuccessSynced = "Synced"
	// IcecreamMessageResourceSynced is the message used for an Event fired when a Icecream
	// is synced successfully
	IcecreamMessageResourceSynced = "Icecream synced successfully"
)

// IcecreamController is the controller implementation for Icecream resources
type IcecreamController struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// sampleclientset is a clientset for our own API group
	sampleclientset clientset.Interface

	podsLister      corelisters.PodLister
	podsSynced      cache.InformerSynced
	icecreamsLister listers.IcecreamLister
	icecreamsSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface
	recorder  record.EventRecorder
}

// NewIcecreamController returns a new icecream controller
func NewIcecreamController(
	kubeclientset kubernetes.Interface,
	sampleclientset clientset.Interface,
	podInformer coreinformers.PodInformer,
	icecreamInformer informers.IcecreamInformer) *IcecreamController {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: icecreamControllerAgentName})

	controller := &IcecreamController{
		kubeclientset:   kubeclientset,
		sampleclientset: sampleclientset,
		podsLister:      podInformer.Lister(),
		podsSynced:      podInformer.Informer().HasSynced,
		icecreamsLister: icecreamInformer.Lister(),
		icecreamsSynced: icecreamInformer.Informer().HasSynced,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Icecreams"),
		recorder:        recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when Icecream resources change
	icecreamInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueIcecream,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueIcecream(new)
		},
	})

	// Set up an event handler for when Pod resources change.
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newPod := new.(*v1.Pod)
			oldPod := old.(*v1.Pod)
			if newPod.ResourceVersion == oldPod.ResourceVersion {
				// Periodic resync will send update events for all known Pods.
				// Two different versions of the same Pod will always have different RVs.
				return
			}
			klog.Infof("Trying to update the pod ...")
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *IcecreamController) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Icecream controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.podsSynced, c.icecreamsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process Icecream resources
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
func (c *IcecreamController) runWorker() {
	for c.processNextWorkItem() {
		klog.Infof("IcecreamController.runWorker: processing next item")
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *IcecreamController) processNextWorkItem() bool {
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
		// workqueue meafns the items in the informer cache may actually be
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
		// Icecream resource to be synced.
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

// syncHandler implements the logic of the controller. It compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Icecream resource with the current status of the resource.
func (c *IcecreamController) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Icecream resource with this namespace/name
	icecream, err := c.icecreamsLister.Icecreams(namespace).Get(name)
	if err != nil {
		// The Icecream resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("icecream '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	flavor := icecream.Spec.Flavor
	if flavor == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		utilruntime.HandleError(fmt.Errorf("%s: icecream's flavor must be specified", key))
		return nil
	}

	// Get selected pods in the same namespace with the matching label based on "flavor"
	matchingLabels := map[string]string{
		"ice-cream-flavor": icecream.Spec.Flavor,
	}
	pods, err := c.podsLister.Pods(namespace).List(labels.SelectorFromSet(labels.Set(matchingLabels)))
	if err != nil {
		klog.Infof("Error while listing matching pods in the same namespace with correct label: %v", err)
		return nil
	}
	if len(pods) == 0 {
		klog.Infof("ERROR - No pods available")
		return nil
	}

	// Finally, we update the status block of the Icecream resource to reflect the
	// current state of the world
	err = c.updateIcecreamStatus(icecream, pods)
	if err != nil {
		return err
	}

	c.recorder.Event(icecream, corev1.EventTypeNormal, IcecreamSuccessSynced, IcecreamMessageResourceSynced)
	return nil
}

func (c *IcecreamController) updateIcecreamStatus(icecream *nicolev1alpha1.Icecream, pods []*v1.Pod) error {
	icecreamCopy := icecream.DeepCopy()

	var totalAllocatedCPU int64 = 0
	var totalAllocatedMemory int64 = 0

	var podIPs []string
	var podNames []string
	for _, pod := range pods {
		var cpuSum int64 = 0
		var memorySum int64 = 0

		podIPs = append(podIPs, pod.Status.PodIP)
		podNames = append(podNames, pod.GetName())

		for _, container := range pod.Spec.Containers {
			if cpuLimit, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
				cpuSum += cpuLimit.Value()
			}

			if memoryLimit, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
				memorySum += memoryLimit.Value()
			}
		}
		icecreamCopy.Status.PodIPs = podIPs
		icecreamCopy.Status.PodNames = podNames
		totalAllocatedCPU += cpuSum
		totalAllocatedMemory += memorySum
	}

	icecreamCopy.Status.TotalAllocatedCPU = strconv.FormatInt(totalAllocatedCPU, 10)
	icecreamCopy.Status.TotalAllocatedMemory = strconv.FormatInt(totalAllocatedMemory, 10)

	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.sampleclientset.ControllerV1alpha1().Icecreams(icecream.Namespace).UpdateStatus(context.TODO(), icecreamCopy, metav1.UpdateOptions{})
	return err
}

// enqueueIcecream takes a Icecream resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Icecream.
func (c *IcecreamController) enqueueIcecream(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Icecream resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Icecream resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *IcecreamController) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by an Icecream, we should not do anything more
		// with it.
		if ownerRef.Kind != "Icecream" {
			return
		}

		icecream, err := c.icecreamsLister.Icecreams(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of icecream '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueIcecream(icecream)
		return
	}
}
