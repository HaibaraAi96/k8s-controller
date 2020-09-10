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
	"reflect"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	icecreamcontroller "k8s.io/k8s-controller/pkg/apis/nicolecontroller/v1alpha1"
	"k8s.io/k8s-controller/pkg/client_icecream/clientset/versioned/fake"
	icecreaminformers "k8s.io/k8s-controller/pkg/client_icecream/informers/externalversions"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type fixture struct {
	t *testing.T

	client     *fake.Clientset
	kubeclient *k8sfake.Clientset
	// Objects to put in the store.
	icecreamLister []*icecreamcontroller.Icecream
	podLister      []*v1.Pod
	// Actions expected to happen on the client.
	kubeactions []core.Action
	actions     []core.Action
	// Objects from here preloaded into NewSimpleFake.
	kubeobjects []runtime.Object
	objects     []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.objects = []runtime.Object{}
	f.kubeobjects = []runtime.Object{}
	return f
}

// Return an icecream resource
func newIcecream(name string, flavor string) *icecreamcontroller.Icecream {
	return &icecreamcontroller.Icecream{
		TypeMeta: metav1.TypeMeta{APIVersion: icecreamcontroller.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: icecreamcontroller.IcecreamSpec{
			Flavor: flavor,
		},
	}
}

// func newIcecreamWithStatus(name string, flavor string) *icecreamcontroller.Icecream {
// 	return &icecreamcontroller.Icecream{
// 		TypeMeta: metav1.TypeMeta{APIVersion: icecreamcontroller.SchemeGroupVersion.String()},
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      name,
// 			Namespace: metav1.NamespaceDefault,
// 		},
// 		Spec: icecreamcontroller.IcecreamSpec{
// 			Flavor: flavor,
// 		},
// 		Status: icecreamcontroller.IcecreamStatus{
// 			PodIPs:               []string{"10.68.72.2"},
// 			PodNames:             []string{"a"},
// 			TotalAllocatedCPU:    "1",
// 			TotalAllocatedMemory: "1048",
// 		},
// 	}
// }

func (f *fixture) expectUpdateIcecreamStatusAction(icecream *icecreamcontroller.Icecream) {
	action := core.NewUpdateAction(schema.GroupVersionResource{Resource: "icecreams"}, icecream.Namespace, icecream)
	action.Subresource = "status"
	f.actions = append(f.actions, action)
}

// Set up our controller, return the controller and informers
func (f *fixture) newController() (*IcecreamController, icecreaminformers.SharedInformerFactory, kubeinformers.SharedInformerFactory) {
	f.client = fake.NewSimpleClientset(f.objects...)
	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobjects...)

	icecreamInformers := icecreaminformers.NewSharedInformerFactory(f.client, noResyncPeriodFunc())
	k8sInformers := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())

	c := NewIcecreamController(f.kubeclient, f.client, k8sInformers.Core().V1().Pods(), icecreamInformers.Controller().V1alpha1().Icecreams())

	c.icecreamsSynced = alwaysReady
	c.podsSynced = alwaysReady
	c.recorder = &record.FakeRecorder{}

	for _, icecream := range f.icecreamLister {
		icecreamInformers.Controller().V1alpha1().Icecreams().Informer().GetIndexer().Add(icecream)
	}

	for _, p := range f.podLister {
		k8sInformers.Core().V1().Pods().Informer().GetIndexer().Add(p)
	}

	return c, icecreamInformers, k8sInformers
}

func (f *fixture) run(icecreamName string) {
	f.runController(icecreamName, true, false)
}

func (f *fixture) runExpectError(icecreamName string) {
	f.runController(icecreamName, true, true)
}

func (f *fixture) runController(icecreamName string, startInformers bool, expectError bool) {
	c, icecreamInformers, k8sInformers := f.newController()
	if startInformers {
		stopCh := make(chan struct{})
		defer close(stopCh)

		icecreamInformers.Start(stopCh)
		k8sInformers.Start(stopCh)

		// icecreamInformers.WaitForCacheSync(stopCh)
		// k8sInformers.WaitForCacheSync(stopCh)

		// if ok := cache.WaitForCacheSync(stopCh, c.podsSynced, c.icecreamsSynced); !ok {
		// 	f.t.Errorf("failed to wait for caches to sync")
		// }
		f.t.Logf("started informers")
	}

	err := c.syncHandler(icecreamName)
	if !expectError && err != nil {
		f.t.Errorf("error syncing icecream: %v", err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing icecream, got nil")
	}

	actions := filterInformerActions(f.client.Actions())
	for i, action := range actions {
		if len(f.actions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(actions)-len(f.actions), actions[i:])
			break
		}

		expectedAction := f.actions[i]
		checkAction(expectedAction, action, f.t)
	}

	if len(f.actions) > len(actions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.actions)-len(actions), f.actions[len(actions):])
	}

	k8sActions := filterInformerActions(f.kubeclient.Actions())
	for i, action := range k8sActions {
		if len(f.kubeactions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(k8sActions)-len(f.kubeactions), k8sActions[i:])
			break
		}

		expectedAction := f.kubeactions[i]
		checkAction(expectedAction, action, f.t)
	}

	if len(f.kubeactions) > len(k8sActions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.kubeactions)-len(k8sActions), f.kubeactions[len(k8sActions):])
	}
}

// checkAction verifies that expected and actual actions are equal and both have
// same attached resources
func checkAction(expected, actual core.Action, t *testing.T) {
	if !(expected.Matches(actual.GetVerb(), actual.GetResource().Resource) && actual.GetSubresource() == expected.GetSubresource()) {
		t.Errorf("Expected\n\t%#v\ngot\n\t%#v", expected, actual)
		return
	}

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Action has wrong type. Expected: %t. Got: %t", expected, actual)
		return
	}

	switch a := actual.(type) {
	case core.CreateActionImpl:
		e, _ := expected.(core.CreateActionImpl)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expObject, object))
		}
	case core.UpdateActionImpl:
		e, _ := expected.(core.UpdateActionImpl)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expObject, object))
		}
	case core.PatchActionImpl:
		e, _ := expected.(core.PatchActionImpl)
		expPatch := e.GetPatch()
		patch := a.GetPatch()

		if !reflect.DeepEqual(expPatch, patch) {
			t.Errorf("Action %s %s has wrong patch\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, diff.ObjectGoPrintSideBySide(expPatch, patch))
		}
	default:
		t.Errorf("Uncaptured Action %s %s, you should explicitly add a case to capture it",
			actual.GetVerb(), actual.GetResource().Resource)
	}
}

// filterInformerActions filters list and watch actions for testing resources.
// Since list and watch don't change resource state we can filter it to lower
// nose level in our tests.
func filterInformerActions(actions []core.Action) []core.Action {
	result := []core.Action{}
	for _, action := range actions {
		if len(action.GetNamespace()) == 0 &&
			(action.Matches("list", "icecreams") ||
				action.Matches("watch", "icecreams") ||
				action.Matches("list", "pods") ||
				action.Matches("watch", "pods")) {
			continue
		}
		result = append(result, action)
	}

	return result
}

func getKey(icecream *icecreamcontroller.Icecream, t *testing.T) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(icecream)
	if err != nil {
		t.Errorf("Unexpected error getting key for icecream %v: %v", icecream.Name, err)
		return ""
	}
	return key
}

func makePod(podName string, phase v1.PodPhase, podIP string, cpu string, memory string, labels map[string]string) *v1.Pod {
	limitCPU := "500m"
	limitMem := "128Mi"

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Labels:    labels,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse(cpu),
							v1.ResourceMemory: resource.MustParse(memory),
						},
						Limits: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse(limitCPU),
							v1.ResourceMemory: resource.MustParse(limitMem),
						},
					},
				},
			},
		},
		Status: v1.PodStatus{
			Phase: phase,
			PodIP: podIP,
		},
	}
}

// Test 1: if we add a pod with a different label, icecream's status will not update
func TestDoNothing(t *testing.T) {
	f := newFixture(t)
	icecream := newIcecream("icecream-test", "chocolate")
	labels := map[string]string{
		"ice-cream-flavor": "mango",
	}
	p := makePod("a", v1.PodRunning, "10.68.72.2", "1", "1048", labels)

	f.icecreamLister = append(f.icecreamLister, icecream)
	f.objects = append(f.objects, icecream)
	f.podLister = append(f.podLister, p)
	f.kubeobjects = append(f.kubeobjects, p)

	f.run(getKey(icecream, t))
}

// Test 2: if we add a pod with matching label, the icecream's status will update accordingly
func TestAddsPod(t *testing.T) {
	f := newFixture(t)

	// original icecream resource
	icecream := newIcecream("icecream-test", "chocolate")
	f.icecreamLister = append(f.icecreamLister, icecream)
	f.objects = append(f.objects, icecream)

	labels := map[string]string{
		"ice-cream-flavor": "chocolate",
	}
	// Add a new pod with a matching label
	p := makePod("a", v1.PodRunning, "10.68.72.2", "1", "1048", labels)
	f.podLister = append(f.podLister, p)
	f.kubeobjects = append(f.kubeobjects, p)

	icecreamCopy := icecream.DeepCopy()
	icecreamCopy.Status.PodIPs = []string{"10.68.72.2"}
	icecreamCopy.Status.PodNames = []string{"a"}
	icecreamCopy.Status.TotalAllocatedCPU = "1"
	icecreamCopy.Status.TotalAllocatedMemory = "1048"

	f.expectUpdateIcecreamStatusAction(icecreamCopy)

	f.run(getKey(icecream, t))
}

// Test 3: Add more pods with matching labels but different running status
func TestUpdatePod(t *testing.T) {
	f := newFixture(t)
	icecream := newIcecream("icecream-test", "chocolate")
	labels := map[string]string{
		"ice-cream-flavor": icecream.Spec.Flavor,
	}
	p1 := makePod("a", v1.PodRunning, "10.68.72.1", "1", "1048", labels)
	p2 := makePod("b", v1.PodPending, "10.68.72.2", "1", "1048", labels)
	p3 := makePod("c", v1.PodFailed, "10.68.72.3", "1", "1048", labels)

	f.icecreamLister = append(f.icecreamLister, icecream)
	f.objects = append(f.objects, icecream)
	f.podLister = append(f.podLister, p1, p2, p3)
	f.kubeobjects = append(f.kubeobjects, p1, p2, p3)

	// expected icecream with updated status
	icecreamCopy := icecream.DeepCopy()
	icecreamCopy.Status.PodIPs = []string{"10.68.72.1", "10.68.72.2", "10.68.72.3"}
	icecreamCopy.Status.PodNames = []string{"a", "b", "c"}
	icecreamCopy.Status.TotalAllocatedCPU = "3"
	icecreamCopy.Status.TotalAllocatedMemory = "3144"

	f.expectUpdateIcecreamStatusAction(icecreamCopy)
	f.run(getKey(icecream, t))
}

// Test 4: if we delete a pod with matching label, the icecream's status will update accordingly
func TestDeletesPod(t *testing.T) {
	f := newFixture(t)

	// original icecream resource
	icecream := newIcecream("icecream-test", "chocolate")
	f.icecreamLister = append(f.icecreamLister, icecream)
	f.objects = append(f.objects, icecream)

	labels := map[string]string{
		"ice-cream-flavor": "chocolate",
	}
	// Add 2 new pods with a matching label
	p1 := makePod("a", v1.PodRunning, "10.68.72.2", "1", "1048", labels)
	p2 := makePod("b", v1.PodRunning, "10.68.72.3", "1", "1048", labels)
	f.podLister = append(f.podLister, p1, p2)
	f.kubeobjects = append(f.kubeobjects, p1, p2)

	// delete the second pod with a maching label
	f.podLister = remove(f.podLister, 1)
	f.kubeobjects = removeObject(f.kubeobjects, 1)

	icecreamCopy2 := icecream.DeepCopy()
	icecreamCopy2.Status.PodIPs = []string{"10.68.72.2"}
	icecreamCopy2.Status.PodNames = []string{"a"}
	icecreamCopy2.Status.TotalAllocatedCPU = "1"
	icecreamCopy2.Status.TotalAllocatedMemory = "1048"

	f.expectUpdateIcecreamStatusAction(icecreamCopy2)

	f.run(getKey(icecream, t))
}

func remove(slice []*v1.Pod, s int) []*v1.Pod {
	return append(slice[:s], slice[s+1:]...)
}

func removeObject(slice []runtime.Object, s int) []runtime.Object {
	return append(slice[:s], slice[s+1:]...)
}
