/*
Copyright 2018 The Kubernetes Authors.

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

package machinedeployment

import (
	"testing"
	"time"

	"github.com/openshift/cluster-api/pkg/apis/cluster/common"
	clusterv1beta1 "github.com/openshift/cluster-api/pkg/apis/cluster/v1beta1"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}

const timeout = time.Second * 5
const pollingInterval = 10 * time.Millisecond

func TestReconcile(t *testing.T) {
	labels := map[string]string{"foo": "bar"}
	instance := &clusterv1beta1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"},
		Spec: clusterv1beta1.MachineDeploymentSpec{
			MinReadySeconds:      int32Ptr(0),
			Replicas:             int32Ptr(2),
			RevisionHistoryLimit: int32Ptr(0),
			Selector:             metav1.LabelSelector{MatchLabels: labels},
			Strategy: &clusterv1beta1.MachineDeploymentStrategy{
				Type: common.RollingUpdateMachineDeploymentStrategyType,
				RollingUpdate: &clusterv1beta1.MachineRollingUpdateDeployment{
					MaxUnavailable: intstrPtr(0),
					MaxSurge:       intstrPtr(1),
				},
			},
			Template: clusterv1beta1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: clusterv1beta1.MachineSpec{
					Versions: clusterv1beta1.MachineVersionInfo{Kubelet: "1.10.3"},
				},
			},
		},
	}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		t.Errorf("error creating new manager: %v", err)
	}
	c = mgr.GetClient()

	r := newReconciler(mgr)
	recFn, requests, errors := SetupTestReconcile(r)
	if err := add(mgr, recFn, r.MachineSetToDeployments); err != nil {
		t.Errorf("error adding controller to manager: %v", err)
	}
	defer close(StartTestManager(mgr, t))

	// Create the MachineDeployment object and expect Reconcile to be called.
	if err := c.Create(context.TODO(), instance); err != nil {
		t.Errorf("error creating instance: %v", err)
	}
	defer c.Delete(context.TODO(), instance)
	expectReconcile(t, requests, errors)

	// Verify that the MachineSet was created.
	machineSets := &clusterv1beta1.MachineSetList{}
	expectInt(t, 1, func(ctx context.Context) int {
		if err := c.List(ctx, &client.ListOptions{}, machineSets); err != nil {
			return -1
		}
		return len(machineSets.Items)
	})

	ms := machineSets.Items[0]
	if r := *ms.Spec.Replicas; r != 2 {
		t.Errorf("replicas was %d not 2", r)
	}
	if k := ms.Spec.Template.Spec.Versions.Kubelet; k != "1.10.3" {
		t.Errorf("kubelet was %q not '1.10.3'", k)
	}

	// Delete a MachineSet and expect Reconcile to be called to replace it.
	if err := c.Delete(context.TODO(), &ms); err != nil {
		t.Errorf("error deleting machineset: %v", err)
	}
	expectReconcile(t, requests, errors)
	expectInt(t, 1, func(ctx context.Context) int {
		if err := c.List(ctx, &client.ListOptions{}, machineSets); err != nil {
			return -1
		}
		return len(machineSets.Items)
	})

	// Scale a MachineDeployment and expect Reconcile to be called
	if err := updateMachineDeployment(c, instance, func(d *clusterv1beta1.MachineDeployment) { d.Spec.Replicas = int32Ptr(5) }); err != nil {
		t.Errorf("error scaling machinedeployment: %v", err)
	}
	if err := c.Update(context.TODO(), instance); err != nil {
		t.Errorf("error updating instance: %v", err)
	}
	expectReconcile(t, requests, errors)
	expectInt(t, 5, func(ctx context.Context) int {
		if err := c.List(ctx, &client.ListOptions{}, machineSets); err != nil {
			return -1
		}
		if len(machineSets.Items) != 1 {
			return -1
		}
		return int(*machineSets.Items[0].Spec.Replicas)
	})

	// Update a MachineDeployment, expect Reconcile to be called and a new MachineSet to appear
	if err := updateMachineDeployment(c, instance, func(d *clusterv1beta1.MachineDeployment) { d.Spec.Template.Labels["updated"] = "true" }); err != nil {
		t.Errorf("error scaling machinedeployment: %v", err)
	}
	if err := c.Update(context.TODO(), instance); err != nil {
		t.Errorf("error updating instance: %v", err)
	}
	expectReconcile(t, requests, errors)
	expectInt(t, 2, func(ctx context.Context) int {
		if err := c.List(ctx, &client.ListOptions{}, machineSets); err != nil {
			return -1
		}
		return len(machineSets.Items)
	})

	// Wait for the new MachineSet to get scaled up and set .Status.Replicas and .Status.AvailableReplicas
	// at each step
	var newMachineSet, oldMachineSet *clusterv1beta1.MachineSet
	if machineSets.Items[0].CreationTimestamp.Before(&machineSets.Items[1].CreationTimestamp) {
		newMachineSet = &machineSets.Items[0]
		oldMachineSet = &machineSets.Items[1]
	} else {
		newMachineSet = &machineSets.Items[1]
		oldMachineSet = &machineSets.Items[0]
	}

	// Start off by setting .Status.Replicas and .Status.AvailableReplicas of the old MachineSet
	oldMachineSet.Status.AvailableReplicas = *oldMachineSet.Spec.Replicas
	oldMachineSet.Status.Replicas = *oldMachineSet.Spec.Replicas
	if err := c.Status().Update(context.TODO(), oldMachineSet); err != nil {
		t.Errorf("error updating machineset: %v", err)
	}
	expectReconcile(t, requests, errors)

	// Iterate over the scalesteps
	for i := 1; i < 6; i++ {
		// Wait for newMachineSet to be scaled up
		expectInt(t, i, func(ctx context.Context) int {
			if err = c.Get(ctx, types.NamespacedName{
				Namespace: newMachineSet.Namespace, Name: newMachineSet.Name}, newMachineSet); err != nil {
				return -1
			}
			return int(*newMachineSet.Spec.Replicas)
		})

		// Set its status
		newMachineSet.Status.Replicas = *newMachineSet.Spec.Replicas
		newMachineSet.Status.AvailableReplicas = *newMachineSet.Spec.Replicas
		if err := c.Status().Update(context.TODO(), newMachineSet); err != nil {
			t.Errorf("error updating machineset: %v", err)
		}
		expectReconcile(t, requests, errors)

		// Wait for oldMachineSet to be scaled down
		expectInt(t, 5-i, func(ctx context.Context) int {
			if err := c.Get(ctx, types.NamespacedName{
				Namespace: oldMachineSet.Namespace, Name: oldMachineSet.Name}, oldMachineSet); err != nil {
				return -1
			}
			return int(*oldMachineSet.Spec.Replicas)
		})

		// Set its status
		oldMachineSet.Status.Replicas = *oldMachineSet.Spec.Replicas
		oldMachineSet.Status.AvailableReplicas = *oldMachineSet.Spec.Replicas
		oldMachineSet.Status.ObservedGeneration = oldMachineSet.Generation
		if err := c.Status().Update(context.TODO(), oldMachineSet); err != nil {
			t.Errorf("error updating machineset: %v", err)
		}
		expectReconcile(t, requests, errors)
	}

	// Expect the old MachineSet to be removed
	expectInt(t, 1, func(ctx context.Context) int {
		if err := c.List(ctx, &client.ListOptions{}, machineSets); err != nil {
			return -1
		}
		return len(machineSets.Items)
	})
}

func int32Ptr(i int32) *int32 {
	return &i
}

func intstrPtr(i int32) *intstr.IntOrString {
	// FromInt takes an int that must not be greater than int32...
	intstr := intstr.FromInt(int(i))
	return &intstr
}

func expectReconcile(t *testing.T, requests chan reconcile.Request, errors chan error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()

LOOP:
	for range time.Tick(pollingInterval) {
		select {
		case recv := <-requests:
			if recv == expectedRequest {
				break LOOP
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting reconcile request")
		}
	}

	for range time.Tick(pollingInterval) {
		select {
		case err := <-errors:
			if err == nil {
				return
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting reconcile error")
		}
	}
}

func expectInt(t *testing.T, expect int, fn func(context.Context) int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()

	for range time.Tick(pollingInterval) {
		intCh := make(chan int)
		go func() { intCh <- fn(ctx) }()

		select {
		case n := <-intCh:
			if n == expect {
				return
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting for value: %d", expect)
			return
		}
	}
}
