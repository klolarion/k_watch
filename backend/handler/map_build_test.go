package handler

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func boolPtr(v bool) *bool { return &v }

func owner(kind, name string) metav1.OwnerReference {
	return metav1.OwnerReference{Kind: kind, Name: name, Controller: boolPtr(true)}
}

func TestSummarizeNamespacesCountsUnhealthyPods(t *testing.T) {
	waiting := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "crash"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			}},
		},
	}
	running := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "ok"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	pending := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "wait"}, Status: corev1.PodStatus{Phase: corev1.PodPending}}
	succeeded := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "batch", Name: "done"}, Status: corev1.PodStatus{Phase: corev1.PodSucceeded}}

	got := SummarizeNamespaces(
		[]corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: "batch"}}, {ObjectMeta: metav1.ObjectMeta{Name: "shop"}}},
		[]corev1.Pod{waiting, running, pending, succeeded},
		[]appsv1.Deployment{{ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "api"}}},
		nil, nil, nil, nil,
		[]corev1.Service{{ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "api"}}},
		nil, nil,
	)
	if len(got) != 2 || got[0].Name != "shop" || got[0].Pods != 3 || got[0].UnhealthyPods != 2 {
		t.Fatalf("shop summary = %+v", got)
	}
	if got[0].Deployments != 1 || got[0].Services != 1 {
		t.Fatalf("workload counts = %+v", got[0])
	}
	if got[1].Name != "batch" || got[1].UnhealthyPods != 0 || got[1].Pods != 1 {
		t.Fatalf("batch summary = %+v", got[1])
	}
}

func TestNamespaceMapCollapsesPodsAndHidesFinishedJobs(t *testing.T) {
	replicas := int32(2)
	objects := nsObjects{
		Deployments: []appsv1.Deployment{{
			ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "api"},
			Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
			Status:     appsv1.DeploymentStatus{AvailableReplicas: 2},
		}},
		ReplicaSets: []appsv1.ReplicaSet{{
			ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "api-abc", OwnerReferences: []metav1.OwnerReference{owner("Deployment", "api")}},
		}},
		Pods: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "api-0", OwnerReferences: []metav1.OwnerReference{owner("ReplicaSet", "api-abc")}},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "api-1", OwnerReferences: []metav1.OwnerReference{owner("ReplicaSet", "api-abc")}},
				Status:     corev1.PodStatus{Phase: corev1.PodPending},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "loose"},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning},
			},
		},
		CronJobs: []batchv1.CronJob{{ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "backup"}}},
		Jobs: []batchv1.Job{
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "backup-active", OwnerReferences: []metav1.OwnerReference{owner("CronJob", "backup")}},
				Status:     batchv1.JobStatus{Active: 1},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "backup-done", OwnerReferences: []metav1.OwnerReference{owner("CronJob", "backup")}},
				Status:     batchv1.JobStatus{Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}},
			},
		},
		PVCs: []corev1.PersistentVolumeClaim{{
			ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "data"},
			Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: "pv-data"},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
		}},
		PVs: []corev1.PersistentVolume{{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-data"},
			Spec: corev1.PersistentVolumeSpec{Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Gi"),
			}},
			Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
		}},
	}
	for i := 0; i < 25; i++ {
		objects.Jobs = append(objects.Jobs, batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:         "shop",
				Name:              "manual-" + time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC).Format("150405"),
				CreationTimestamp: metav1.NewTime(time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC)),
			},
			Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}},
		})
	}

	got := BuildNamespaceMap("shop", objects)
	if got.HiddenJobs != 6 {
		t.Fatalf("hidden jobs = %d, want 6 (1 cron + 5 standalone)", got.HiddenJobs)
	}
	for _, node := range got.Nodes {
		if node.Type == "Pod" || node.Type == "ReplicaSet" || node.Type == "Node" {
			t.Fatalf("map included %s %s", node.Type, node.ID)
		}
		if node.ID == "job:shop/backup-done" {
			t.Fatalf("finished cron job is on the map")
		}
	}
	deploy := findMapNode(t, got.Nodes, "deploy:shop/api")
	if deploy.Pods == nil || deploy.Pods.Total != 2 || deploy.Pods.Running != 1 || deploy.Pods.Pending != 1 {
		t.Fatalf("deployment pod counts = %+v", deploy.Pods)
	}
	if !hasMapEdge(got.Edges, "cronjob:shop/backup", "job:shop/backup-active", "owns") {
		t.Fatalf("missing cron ownership edge: %+v", got.Edges)
	}
	if !hasMapEdge(got.Edges, "pv:pv-data", "pvc:shop/data", "binds") {
		t.Fatalf("missing pv bind: %+v", got.Edges)
	}
	if findMapNode(t, got.Nodes, "podgroup:shop/unmanaged").Pods.Total != 1 {
		t.Fatal("unmanaged pod was not grouped")
	}
	jobNodes := 0
	for _, node := range got.Nodes {
		if node.Type == "Job" {
			jobNodes++
		}
	}
	if jobNodes != 21 {
		t.Fatalf("job nodes = %d, want 21 (1 active cron job + 20 finished standalone)", jobNodes)
	}
}

func TestFocusDrawsRoutesWithoutListingEveryPod(t *testing.T) {
	objects := nsObjects{
		Deployments: []appsv1.Deployment{{ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "api"}}},
		ReplicaSets: []appsv1.ReplicaSet{{
			ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "api-abc", OwnerReferences: []metav1.OwnerReference{owner("Deployment", "api")}},
		}},
		Services: []corev1.Service{{
			ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "api"},
			Spec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{Port: 80, Protocol: corev1.ProtocolTCP}},
			},
		}},
		Ingresses: []networkingv1.Ingress{{
			ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "shop"},
			Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{
				Host: "shop.example.com",
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{{
						Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "api"}},
					}},
				}},
			}}},
		}},
		EndpointSlices: []discoveryv1.EndpointSlice{{
			ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "api-slice", Labels: map[string]string{"kubernetes.io/service-name": "api"}},
			Endpoints: []discoveryv1.Endpoint{{
				TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "api-0000", Namespace: "shop"},
			}},
		}},
	}
	for i := 0; i < 60; i++ {
		objects.Pods = append(objects.Pods, corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "shop",
				Name:            "api-" + time.Date(2026, 1, 1, 0, i, 0, 0, time.UTC).Format("1504"),
				OwnerReferences: []metav1.OwnerReference{owner("ReplicaSet", "api-abc")},
			},
			Spec:   corev1.PodSpec{NodeName: "worker"},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		})
	}
	objects.Pods[0].Spec.NodeName = "worker-a"
	objects.Pods[0].Status.Phase = corev1.PodFailed

	got, err := BuildFocus("shop", "Deployment", "api", objects)
	if err != nil {
		t.Fatal(err)
	}
	if got.PodTotal != 60 || !got.PodsTruncated || len(got.Pods) != focusPodLimit {
		t.Fatalf("pod list = total %d truncated %v len %d", got.PodTotal, got.PodsTruncated, len(got.Pods))
	}
	if got.Pods[0].Phase != string(corev1.PodFailed) {
		t.Fatalf("failed pod should sort first, got %s", got.Pods[0].Phase)
	}
	if len(got.Nodes) != 2 {
		t.Fatalf("node placements = %+v", got.Nodes)
	}
	if !hasMapEdge(got.Routes, "ing:shop/shop", "svc:shop/api", "routes") || !hasMapEdge(got.Routes, "svc:shop/api", "deploy:shop/api", "routes") {
		t.Fatalf("routes = %+v", got.Routes)
	}
	if _, err := BuildFocus("shop", "Deployment", "missing", objects); err != ErrNotFound {
		t.Fatalf("missing workload error = %v", err)
	}
}

func TestBuildFocusWithoutServiceSendsEmptyArrays(t *testing.T) {
	objects := nsObjects{
		Deployments: []appsv1.Deployment{{ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "worker"}}},
	}
	got, err := BuildFocus("shop", "Deployment", "worker", objects)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"pods":[]`, `"services":[]`, `"ingresses":[]`, `"nodes":[]`, `"routes":[]`, `"highlight":[`} {
		if !strings.Contains(string(body), field) {
			t.Fatalf("response should contain %s, got %s", field, body)
		}
	}
}

func findMapNode(t *testing.T, nodes []MapNode, id string) MapNode {
	t.Helper()
	for _, node := range nodes {
		if node.ID == id {
			return node
		}
	}
	t.Fatalf("missing node %s", id)
	return MapNode{}
}

func hasMapEdge(edges []MapEdge, from, to, kind string) bool {
	for _, edge := range edges {
		if edge.From == from && edge.To == to && edge.Kind == kind {
			return true
		}
	}
	return false
}
