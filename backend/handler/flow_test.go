package handler

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func TestDescribePodFlowRollsUpToDeployment(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "shop",
			Name:            "api-0",
			OwnerReferences: []metav1.OwnerReference{owner("ReplicaSet", "api-abc")},
		},
		Spec: corev1.PodSpec{NodeName: "worker-a"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				RestartCount: 2,
				State:        corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			}},
		},
	}
	idx := ownerIndex{rsDeploy: map[string]string{"api-abc": "api"}, jobCron: map[string]string{}}
	got := DescribePodFlow(string(watch.Added), pod, idx)
	if got.WorkloadID != "deploy:shop/api" || got.WorkloadKind != "Deployment" || got.WorkloadName != "api" {
		t.Fatalf("workload = %+v", got)
	}
	if got.Phase != "Running" || got.Reason != "CrashLoopBackOff" || got.Restarts != 2 || got.Node != "worker-a" {
		t.Fatalf("status = %+v", got)
	}
}

func TestDescribePodFlowMarksDeletion(t *testing.T) {
	now := metav1.NewTime(time.Now())
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "api-0", DeletionTimestamp: &now},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	got := DescribePodFlow(string(watch.Modified), pod, ownerIndex{rsDeploy: map[string]string{}, jobCron: map[string]string{}})
	if !got.Deleting || got.WorkloadID != "podgroup:shop/unmanaged" {
		t.Fatalf("flow = %+v", got)
	}
}

func TestShouldEmitFlowSkipsUnchangedStatus(t *testing.T) {
	sig := "Running||worker-a|0|false|deploy:shop/api"
	if shouldEmitFlow(string(watch.Modified), sig, sig) {
		t.Fatal("unchanged status should not emit")
	}
	if !shouldEmitFlow(string(watch.Modified), sig, "Pending||worker-a|0|false|deploy:shop/api") {
		t.Fatal("phase change should emit")
	}
	if !shouldEmitFlow(string(watch.Added), sig, sig) || !shouldEmitFlow(string(watch.Deleted), sig, sig) {
		t.Fatal("create and delete should always emit")
	}
}
