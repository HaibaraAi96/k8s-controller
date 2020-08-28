# k8s-controller

Write a Kubernetes custom controllers in Go

The controller for a resource are responsible for making the current state come closer to the desired state. It is an active reconciliation process that watches the
shared state of the cluster through the API server and makes changes to move the current state towards the desired state.

In Kubernetes, a controller will send messages to the API server that have useful side effects.

Kubernetes comes with a set of built-in controllers that run inside the kube-controller-manager. These built-in controllers provide important core behaviors.

##### This repo shows how to build a custom controller that watches changes for Pods and log what node it's scheduled on

## The approach
We create a custom controller that does the following:

- Continuously watch the API server for Pods changes.
- When it detects a change in our ConfigMap it searches for all the Pods that have app=frontend label and deletes them.
- Because our application is deployed through a Deployment controller, all deleted Pods are automatically started again.



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
To confirm that our custom controller is working, we need to make and apply a change to the ConfigMap. 