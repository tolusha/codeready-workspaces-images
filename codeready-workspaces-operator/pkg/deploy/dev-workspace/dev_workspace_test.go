//
// Copyright (c) 2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package devworkspace

import (
	"context"
	"os"
	"strings"

	"testing"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DevWorkspaceCSVName = "devworkspace-operator.v0.6.0"
)

func TestReconcileDevWorkspace(t *testing.T) {
	type testCase struct {
		name         string
		IsOpenShift  bool
		IsOpenShift4 bool
		cheCluster   *orgv1.CheCluster
	}

	testCases := []testCase{
		{
			name: "Reconcile DevWorkspace on OpenShift",
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
						Enable: true,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(true),
					},
					Server: orgv1.CheClusterSpecServer{
						ServerExposureStrategy: "single-host",
					},
				},
			},
			IsOpenShift:  true,
			IsOpenShift4: true,
		},
		{
			name: "Reconcile DevWorkspace on K8S multi-host",
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
						Enable: true,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(true),
					},
					Server: orgv1.CheClusterSpecServer{
						ServerExposureStrategy: "multi-host",
					},
					K8s: orgv1.CheClusterSpecK8SOnly{
						IngressDomain: "che.domain",
					},
				},
			},
			IsOpenShift:  false,
			IsOpenShift4: false,
		},
		{
			name: "Reconcile DevWorkspace on K8S single-host",
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
						Enable: true,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(true),
					},
					Server: orgv1.CheClusterSpecServer{
						ServerExposureStrategy: "single-host",
					},
					K8s: orgv1.CheClusterSpecK8SOnly{
						IngressDomain: "che.domain",
					},
				},
			},
			IsOpenShift:  false,
			IsOpenShift4: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := deploy.GetTestDeployContext(testCase.cheCluster, []runtime.Object{})
			deployContext.ClusterAPI.Scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.Subscription{})

			util.IsOpenShift = testCase.IsOpenShift
			util.IsOpenShift4 = testCase.IsOpenShift4
			os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")

			done, err := ReconcileDevWorkspace(deployContext)
			if err != nil {
				t.Fatalf("Error: %v", err)
			}
			if !done {
				t.Fatalf("Dev Workspace operator has not been provisioned")
			}
		})
	}
}

func TestShouldNotReconcileDevWorkspaceIfForbiden(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
		},
	}

	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{})

	util.IsOpenShift = true
	util.IsOpenShift4 = true
	os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "false")

	reconciled, err := ReconcileDevWorkspace(deployContext)

	assert.False(t, reconciled, "Devworkspace should not be reconciled")
	assert.NotNil(t, err, "Error expected")
	assert.True(t, strings.Contains(err.Error(), "deploy CodeReady Workspaces from tech-preview channel"), "Unrecognized error occurred %v", err)
}

func TestShouldReconcileDevWorkspaceIfDevWorkspaceDeploymentExists(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
		},
	}

	devworkspaceDeployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      DevWorkspaceDeploymentName,
			Namespace: DevWorkspaceNamespace,
		},
	}

	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{devworkspaceDeployment})

	util.IsOpenShift = true
	util.IsOpenShift4 = true
	os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "false")

	reconciled, err := ReconcileDevWorkspace(deployContext)

	assert.Nil(t, err, "Reconciliation error occurred %v", err)
	assert.True(t, reconciled, "Devworkspace should be reconciled.")
}

func TestReconcileDevWorkspaceShouldThrowErrorIfWebTerminalSubscriptionExists(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
			Auth: orgv1.CheClusterSpecAuth{
				OpenShiftoAuth: util.NewBoolPointer(true),
			},
			Server: orgv1.CheClusterSpecServer{
				ServerExposureStrategy: "single-host",
			},
		},
	}
	subscription := &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WebTerminalOperatorSubscriptionName,
			Namespace: WebTerminalOperatorNamespace,
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{},
	}
	webhook := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceWebhookName,
		},
	}

	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{subscription, webhook})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.Subscription{})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(admissionregistrationv1.SchemeGroupVersion, &admissionregistrationv1.MutatingWebhookConfiguration{})
	deployContext.ClusterAPI.DiscoveryClient.(*fakeDiscovery.FakeDiscovery).Fake.Resources = []*metav1.APIResourceList{
		{
			APIResources: []metav1.APIResource{
				{Name: SubscriptionResourceName},
			},
		},
	}
	util.IsOpenShift = true
	util.IsOpenShift4 = true
	os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")
	_, err := ReconcileDevWorkspace(deployContext)

	if err == nil || err.Error() != "a non matching version of the Dev Workspace operator is already installed" {
		t.Fatalf("Error should be thrown")
	}
}

func TestReconcileDevWorkspaceCheckIfCSVExists(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
			Auth: orgv1.CheClusterSpecAuth{
				OpenShiftoAuth: util.NewBoolPointer(true),
			},
			Server: orgv1.CheClusterSpecServer{
				ServerExposureStrategy: "single-host",
			},
		},
	}
	devWorkspaceCSV := &operatorsv1alpha1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DevWorkspaceCSVName,
			Namespace: "openshift-operators",
		},
		Spec: operatorsv1alpha1.ClusterServiceVersionSpec{},
	}

	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.ClusterServiceVersion{})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.ClusterServiceVersionList{})
	deployContext.ClusterAPI.Client.Create(context.TODO(), devWorkspaceCSV)
	deployContext.ClusterAPI.DiscoveryClient.(*fakeDiscovery.FakeDiscovery).Fake.Resources = []*metav1.APIResourceList{
		{
			APIResources: []metav1.APIResource{
				{
					Name: ClusterServiceVersionResourceName,
				},
			},
		},
	}

	util.IsOpenShift = true
	util.IsOpenShift4 = true
	os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")
	reconciled, _ := ReconcileDevWorkspace(deployContext)

	if !reconciled {
		t.Fatalf("Failed to reconcile DevWorkspace")
	}

	// Get Devworkspace namespace. If error is thrown means devworkspace is not anymore installed if CSV is detected
	err := deployContext.ClusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: DevWorkspaceNamespace}, &corev1.Namespace{})
	if err == nil {
		t.Fatal("Failed to reconcile DevWorkspace when DWO CSV is exptected to be created")
	}
}

func TestShouldSyncNewObject(t *testing.T) {
	deployContext := deploy.GetTestDeployContext(nil, []runtime.Object{})

	newObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{}, "test")
	obj2sync := &Object2Sync{
		obj:     newObject,
		hash256: "hash",
	}

	// tries to sync a new object
	done, err := syncObject(deployContext, obj2sync, "eclipse-che")
	if err != nil {
		t.Fatalf("Failed to sync object: %v", err)
	} else if !done {
		t.Fatalf("Object is not synced.")
	}

	// reads object and check content, object is supposed to be created
	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(deployContext, "test", actual)
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	} else if !exists {
		t.Fatalf("Object not found")
	}

	if actual.GetAnnotations()[deploy.CheEclipseOrgHash256] != "hash" {
		t.Fatalf("Invalid hash")
	}
	if actual.GetAnnotations()[deploy.CheEclipseOrgNamespace] != "eclipse-che" {
		t.Fatalf("Invalid namespace")
	}
}

func TestShouldSyncObjectIfItWasCreatedByAnotherOriginHashDifferent(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
		},
	}
	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{})

	// creates initial object
	initialObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "b"}, "test")
	initialObject.SetAnnotations(map[string]string{
		deploy.CheEclipseOrgHash256:   "hash2",
		deploy.CheEclipseOrgNamespace: "eclipse-che-2",
	})
	deploy.Create(deployContext, initialObject)

	// tries to sync object with a new hash but different origin
	newObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "c"}, "test")
	obj2sync := &Object2Sync{
		obj:     newObject,
		hash256: "hash",
	}
	_, err := syncObject(deployContext, obj2sync, "eclipse-che")
	if err != nil {
		t.Fatalf("Failed to sync object: %v", err)
	}

	// reads object and check content, object supposed to be updated
	// there is only one operator to mange DW resources
	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(deployContext, "test", actual)
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	} else if !exists {
		t.Fatalf("Object not found")
	}

	if actual.GetAnnotations()[deploy.CheEclipseOrgHash256] != "hash" {
		t.Fatalf("Invalid hash")
	}
	if actual.GetAnnotations()[deploy.CheEclipseOrgNamespace] != "eclipse-che" {
		t.Fatalf("Invalid namespace")
	}
	if actual.Data["a"] != "c" {
		t.Fatalf("Invalid data")
	}
}

func TestShouldSyncObjectIfItWasCreatedBySameOriginHashDifferent(t *testing.T) {
	deployContext := deploy.GetTestDeployContext(nil, []runtime.Object{})

	initialObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "b"}, "test")
	initialObject.SetAnnotations(map[string]string{
		deploy.CheEclipseOrgHash256:   "hash",
		deploy.CheEclipseOrgNamespace: "eclipse-che",
	})
	deploy.Create(deployContext, initialObject)

	// creates initial object
	newObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "c"}, "test")
	obj2sync := &Object2Sync{
		obj:     newObject,
		hash256: "newHash",
	}

	// tries to sync object with a new
	_, err := syncObject(deployContext, obj2sync, "eclipse-che")
	if err != nil {
		t.Fatalf("Failed to sync object: %v", err)
	}

	// reads object and check content, object supposed to be updated
	// it was created by the same origin
	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(deployContext, "test", actual)
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	} else if !exists {
		t.Fatalf("Object not found")
	}

	if actual.GetAnnotations()[deploy.CheEclipseOrgHash256] != "newHash" {
		t.Fatalf("Invalid hash")
	}
	if actual.GetAnnotations()[deploy.CheEclipseOrgNamespace] != "eclipse-che" {
		t.Fatalf("Invalid namespace")
	}
	if actual.Data["a"] != "c" {
		t.Fatalf("Invalid data")
	}
}

func TestShouldNotSyncObjectIfThereIsAnotherCheCluster(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che-1",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
		},
	}
	anotherCheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che-2",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
		},
	}
	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{anotherCheCluster})

	// creates initial object
	initialObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "b"}, "test")
	initialObject.SetAnnotations(map[string]string{
		deploy.CheEclipseOrgHash256:   "hash-2",
		deploy.CheEclipseOrgNamespace: "eclipse-che-2",
	})
	deploy.Create(deployContext, initialObject)

	// tries to sync object with a new hash but different origin
	newObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "c"}, "test")
	obj2sync := &Object2Sync{
		obj:     newObject,
		hash256: "hash-1",
	}
	done, err := syncObject(deployContext, obj2sync, "eclipse-che")
	if err != nil {
		t.Fatalf("Failed to sync object: %v", err)
	} else if !done {
		t.Fatalf("Object is not synced.")
	}

	// reads object and check content, object isn't supposed to be updated
	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(deployContext, "test", actual)
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	} else if !exists {
		t.Fatalf("Object not found")
	}

	if actual.GetAnnotations()[deploy.CheEclipseOrgHash256] != "hash-2" {
		t.Fatalf("Invalid hash")
	}
	if actual.GetAnnotations()[deploy.CheEclipseOrgNamespace] != "eclipse-che-2" {
		t.Fatalf("Invalid namespace")
	}
	if actual.Data["a"] != "b" {
		t.Fatalf("Invalid data")
	}
}

func TestShouldNotSyncObjectIfHashIsEqual(t *testing.T) {
	deployContext := deploy.GetTestDeployContext(nil, []runtime.Object{})

	// creates initial object
	initialObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "b"}, "test")
	initialObject.SetAnnotations(map[string]string{
		deploy.CheEclipseOrgHash256:   "hash",
		deploy.CheEclipseOrgNamespace: "eclipse-che",
	})
	deploy.Create(deployContext, initialObject)

	// tries to sync object with the same hash
	newObject := deploy.GetConfigMapSpec(deployContext, "test", map[string]string{"a": "c"}, "test")
	obj2sync := &Object2Sync{
		obj:     newObject,
		hash256: "hash",
	}
	done, err := syncObject(deployContext, obj2sync, "eclipse-che")
	if err != nil {
		t.Fatalf("Failed to sync object: %v", err)
	} else if !done {
		t.Fatalf("Object is not synced.")
	}

	// reads object and check content, object isn't supposed to be updated
	actual := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(deployContext, "test", actual)
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	} else if !exists {
		t.Fatalf("Object not found")
	}

	if actual.GetAnnotations()[deploy.CheEclipseOrgHash256] != "hash" {
		t.Fatalf("Invalid hash")
	}
	if actual.GetAnnotations()[deploy.CheEclipseOrgNamespace] != "eclipse-che" {
		t.Fatalf("Invalid namespace")
	}
	if actual.Data["a"] != "b" {
		t.Fatalf("Object is not supposed to be updated.")
	}
}
