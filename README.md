# k8s-controller

Write a Kubernetes custom controllers in Go

The controller for a resource are responsible for making the current state come closer to the desired state. It is an active reconciliation process that watches the
shared state of the cluster through the API server and makes changes to move the current state towards the desired state.

In Kubernetes, a controller will send messages to the API server that have useful side effects.

Kubernetes comes with a set of built-in controllers that run inside the kube-controller-manager. These built-in controllers provide important core behaviors.



## client-go
The [client-go](https://github.com/kubernetes/client-go/blob/master/INSTALL.md#enabling-go-modules) library provides access to Kubernetes RESTful API interface served by the Kubernetes API server.

We will use client-go library to get access to k8s RESTful API interface served by k8s API server.

```
$ go mod init goelster
$ go get k8s.io/client-go@v0.17.0
```
At this point, I have go.mod and go.sum in my repo

## Controller structure
