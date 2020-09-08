# k8s-controller

Write a Kubernetes custom controllers in Go

Controllers are an essential part of Kubernetes. They are the “brains” behind the resources themselves. 

The controller for a resource are responsible for making the current state come closer to the desired state. It is an active reconciliation process that watches the shared state of the cluster through the API server and makes changes to move the current state towards the desired state.

In Kubernetes, a controller will send messages to the API server that have useful side effects.

Kubernetes comes with a set of built-in controllers that run inside the kube-controller-manager. These built-in controllers provide important core behaviors. But one of the really great features of Kubernetes is the ability to extend it.

##### This repo shows how to build a custom controller that watches changes for Pods and update custom resource status based on pods information

This repository implements a simple controller for watching **Icecream** resources as defined with a CustomResourceDefinition (CRD).

**Note:** go-get or vendor this package as `k8s.io/sample-controller`.

This particular example demonstrates how to perform basic operations such as:

* How to register a new custom resource (custom resource type) of type `Icecream` using a CustomResourceDefinition.
* How to create/get/list instances of your new resource type `Icecream`.
* How to setup a controller on resource handling create/update/delete events.

It makes use of the generators in [k8s.io/code-generator](https://github.com/kubernetes/code-generator)
to generate a typed client, informers, listers and deep-copy functions. 

By using the `./hack/update-codegen.sh` script, we automatically generate the following files &
directories:

* `pkg/apis/samplecontroller/v1alpha1/zz_generated.deepcopy.go`
* `pkg/generated/`

Changes should not be made to these files manually, and when creating your own
controller based off of this implementation you should not copy these files and
instead run the `update-codegen` script to generate your own.

## Details



## Fetch sample-controller and its dependencies

### When using go 1.11 modules

When using go 1.11 modules (`GO111MODULE=on`), issue the following
commands --- starting in whatever working directory you like.

```sh
git clone https://github.com/kubernetes/sample-controller.git
cd sample-controller
```

- You also need to clone the **code-generator repo** to exist in $GOPATH/k8s.io/  
- An alternative way to do this is to use the command `go mod vendor` to create and populate the `vendor` directory


## Purpose

This is an example of how to build a kube-like controller with a single type.

## Running

**Prerequisite**: Since the sample-controller uses `apps/v1` deployments, the Kubernetes cluster version should be greater than 1.9.

```sh
# assumes you have a working kubeconfig, not required if operating in-cluster
go build -o sample-controller .
./sample-controller -kubeconfig=$HOME/.kube/config

# create a CustomResourceDefinition
kubectl create -f artifacts/examples/crd.yaml

# create a custom resource of type Foo
kubectl create -f artifacts/examples/example-foo.yaml

# check deployments created through the custom resource
kubectl get deployments
```

## Use Cases

CustomResourceDefinitions can be used to implement custom resource types for your Kubernetes cluster.
These act like most other Resources in Kubernetes, and may be `kubectl apply`'d, etc.

Some example use cases:

* Provisioning/Management of external datastores/databases (eg. CloudSQL/RDS instances)
* Higher level abstractions around Kubernetes primitives (eg. a single Resource to define an etcd cluster, backed by a Service and a ReplicationController)

## Defining types

Each instance of your custom resource has an attached Spec, which should be defined via a `struct{}` to provide data format validation.
In practice, this Spec is arbitrary key-value data that specifies the configuration/behavior of your Resource.

For example, if you were implementing a custom resource for an Icecream, you might provide a IcecreamSpec like the following:

``` go
// Icecream is a specification for a Icecream resource
type Icecream struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IcecreamSpec   `json:"spec"`
	Status IcecreamStatus `json:"status"`
}

// IcecreamSpec is the spec for an Icecream resource
type IcecreamSpec struct {
	DeploymentName string `json:"deploymentName"`
	Replicas       *int32 `json:"replicas"`
}
```


## Cleanup

You can clean up the created CustomResourceDefinition with:

    kubectl delete crd icecreams.controller.nicoleh.io

## Compatibility

HEAD of this repository will match HEAD of k8s.io/apimachinery and
k8s.io/client-go.

## Where does it come from?

`sample-controller` is cloned from
https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/sample-controller.

<br>

## The approach
We create a custom controller that does the following:

- Continuously watch the API server for Pods changes
- Log what node it's scheduled on for our customized resource "icecream"

## Controller overview
Kubernetes has a very “pluggable” way to add your own logic in the form of a controller. A controller is a component that you can develop and run in the context of a Kubernetes cluster.

Controllers are an essential part of Kubernetes. They are the “brains” behind the resources themselves.

You can have a custom controller without a custom resource (e.g. custom logic on native resource types like Pods and Deployments). Conversely, you can have custom resources without a controller, but that is a glorified data store with no custom logic behind it.

## Controller event flow
The controller “subscribes” to a queue. The controller worker is going to block on a call to get the next item from the queue.

An event is the combination of an action (create, update, or delete) and a resource key (typically in the format of namespace/name).

The informer is the “link” to the part of Kubernetes that is tasked with handing out these events, as well as retrieving the resources in the cluster to focus on. Put another way, the informer is the proxy between Kubernetes and your controller (and the queue is the store for it).

Part of the informer’s responsibility is to register event handlers for the three different types of events: Add, update, and delete. It is in those informer’s event handler functions that we add the key to the queue to pass off logic to the controller’s handlers.


## Controller: Core resources v.s. Custom resources
There are two types of resources that controllers can “watch”: Core resources and custom resources. Core resources are what Kubernetes ship with (for instance: Pods).

Handling core resource events is interesting, and a great way to understand the basic mechanisms of controllers, informers, and queues. But the use-cases are limited. The real power and flexibility with controllers is when you can start working with custom resources.

You can think of custom resources as the data, and controllers as the logic behind the data. Working together, they are a significant component to extending Kubernetes.


## client-go
The library contains several important packages and utilities which can be used for accessing the API resources or facilitate a custom controller.

The [client-go](https://github.com/kubernetes/client-go/blob/master/INSTALL.md#enabling-go-modules) library provides access to Kubernetes RESTful API interface served by the Kubernetes API server. Tools like kubectl or prometheus-operator use it intensively.

We will use client-go library to get access to k8s RESTful API interface served by k8s API server.

```
$ go mod init goelster
$ go get k8s.io/client-go@v0.17.0
```
At this point, I have go.mod and go.sum in my repo


## Controller structure
The simplest implementation of a controller is a loop:
```go
for {
  desired := getDesiredState()
  current := getCurrentState()
  makeChanges(desired, current)
}
```
Main tasks in the controller workflow:

1. Use informers to keep track of add/update/delete events for the Kubernetes resources that we want to know about. “Keeping track” involves storing them in a local cache (thread-safe store) and also adding them to a workqueue.

2. Consume items from the workqueue and process them.

There are two main components of a controller: Informer and Workqueue.

#### Informer
- Informer watches for changes on the current state of Kubernetes objects and sends events to Workqueue where events are then popped up by worker(s) to process
- In order to retrieve an object's information, the controller sends a request to Kubernetes API server
- In order to get and list objects multiple times with less cost, we use cache provided by client-go library
- The controller does not really want to send requests continuously. It only cares about events when the object has been created, modified or deleted. The client-go library provides the **Listwatcher interface** that performs an initial list and starts a watch on a particular resource

There are several patterns used to construct the Informer, in this example, we use the pattern of **Resource Event Handler**

##### RESOURCE EVENT HANDLER
Resource Event Handler is where the controller handles notifications for changes on a particular resource:
```go
type ResourceEventHandlerFuncs struct {
	AddFunc    func(obj interface{})
	UpdateFunc func(oldObj, newObj interface{})
	DeleteFunc func(obj interface{})
}
```
- AddFunc is called when a new resource is created.
- UpdateFunc is called when an existing resource is modified. The oldObj is the last known state of the resource. UpdateFunc is also called when a re-synchronization happens, and it gets called even if nothing changes.
- DeleteFunc is called when an existing resource is deleted. It gets the final state of the resource (if it is known). Otherwise, it gets an object of type DeletedFinalStateUnknown. This can happen if the watch is closed and misses the delete event and the controller doesn't notice the deletion until the subsequent re-list.

<br><br>

#### Workqueue

- provided by client-go library at client-go/util/workqueue. There are several kinds of queues supported including the delayed queue, timed queue and rate limiting queue
- Whenever a resource changes, the Resource Event Handler puts a key to the Workqueue. A key uses the format <resource_namespace>/<resource_name>
- Events are collapsed by key so each consumer can use worker(s) to pop key up and this work is done sequentially. This will gurantee that no two workers will work on the same key at the same time

The following is an example for creating a rate limiting queue:
```go
queue :=
workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
```

If processing an event fails, the controller calls the **AddRateLimited()** function to push the key back to the workqueue to work on later with a predefined number of retries. Otherwise, if the process is successful, the key can be removed from the workqueue by calling the **Forget()** function. However, that function only stops the workqueue from tracking the history of the event. In order to remove the event completely from the workqueue, the controller must trigger the **Done()** function.
<br>

So the workqueue can handle notifications from cache, but the question is, when should the controller start workers processing the workqueue? 

There are two reasons that the controller should wait until **the cache is completely synchronized** in order to achieve the latest states:

1. Listing all the resources will be inaccurate until the cache has finished synchronising

2. Multiple rapid updates to a single resource will be collapsed into the latest version by the cache/queue. Therefore, it must wait until the cache becomes idle before actually processing items to avoid wasted work on intermediate states

#### After we have an overview of the elements involved in a Kubernetes controller, we can start writing a Kubernetes custom controller!

## Testing Our Work
To confirm that our custom controller is working, we need to run:
```sh
$ kubectl apply -f artifacts/examples/crd-icecream.yaml

$ kubectl apply -f artifacts/examples/example-icecream.yaml

# we can view the resource
$ kubectl get icecream
NAME               AGE
example-icecream   32s

# run our application
$ go build -o k8s-controller .
$ ./k8s-controller -kubeconfig=$HOME/.kube/config
I0831 13:28:06.675486   26724 icecream_controller.go:108] Setting up event handlers
I0831 13:28:06.675594   26724 icecream_controller.go:148] Starting Icecream controller
I0831 13:28:06.675598   26724 icecream_controller.go:151] Waiting for informer caches to sync
I0831 13:28:06.975692   26724 icecream_controller.go:156] Starting workers
I0831 13:28:06.975714   26724 icecream_controller.go:162] Started workers
I0831 13:28:06.975748   26724 icecream_controller.go:299] HERE - NodeName: nicole-icecream
I0831 13:28:07.049454   26724 icecream_controller.go:221] Successfully synced 'default/example-icecream'
I0831 13:28:07.049467   26724 icecream_controller.go:174] IcecreamController.runWorker: processing next item
I0831 13:28:07.049475   26724 icecream_controller.go:299] HERE - NodeName: nicole-icecream
I0831 13:28:07.049810   26724 event.go:291] "Event occurred" object="default/example-icecream" kind="Icecream" apiVersion="controller.nicoleh.io/v1alpha1" type="Normal" reason="Synced" message="Icecream synced successfully"
I0831 13:28:07.120629   26724 icecream_controller.go:221] Successfully synced 'default/example-icecream'
I0831 13:28:07.120646   26724 icecream_controller.go:174] IcecreamController.runWorker: processing next item
I0831 13:28:07.120711   26724 event.go:291] "Event occurred" object="default/example-icecream" kind="Icecream" apiVersion="controller.nicoleh.io/v1alpha1" type="Normal" reason="Synced" message="Icecream synced successfully"
^CI0831 13:28:27.698249   26724 icecream_controller.go:164] Shutting down workers
# we can see the nodeName is "nicole-icecream"

$ kubectl get deployment
NAME                                  READY   UP-TO-DATE   AVAILABLE   AGE
example-icecream                      1/1     1            1           16s
```
