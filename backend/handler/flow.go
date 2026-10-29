package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/klolarion/k_watch/backend/k8s"
)

var errWatchExpired = errors.New("watch expired")

// PodFlow is one pod lifecycle change. The map draws it under the owning workload
// and drops it after a few seconds, so the canvas does not keep every pod.
type PodFlow struct {
	Type         string    `json:"type"`
	Time         time.Time `json:"time"`
	Pod          string    `json:"pod"`
	Phase        string    `json:"phase"`
	Reason       string    `json:"reason,omitempty"`
	Node         string    `json:"node,omitempty"`
	Restarts     int32     `json:"restarts"`
	Deleting     bool      `json:"deleting"`
	WorkloadID   string    `json:"workloadId"`
	WorkloadKind string    `json:"workloadKind"`
	WorkloadName string    `json:"workloadName"`
}

func DescribePodFlow(eventType string, pod *corev1.Pod, idx ownerIndex) PodFlow {
	ref, ok := resolveWorkload(*pod, idx)
	if !ok {
		ref = workloadRef{Kind: "PodGroup", Name: "unmanaged"}
	}
	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}
	return PodFlow{
		Type:         eventType,
		Pod:          pod.Name,
		Phase:        string(pod.Status.Phase),
		Reason:       podWaitReason(pod),
		Node:         pod.Spec.NodeName,
		Restarts:     restarts,
		Deleting:     pod.DeletionTimestamp != nil,
		WorkloadID:   nodeID(ref.Kind, pod.Namespace, ref.Name),
		WorkloadKind: ref.Kind,
		WorkloadName: ref.Name,
	}
}

func podWaitReason(pod *corev1.Pod) string {
	for _, cs := range pod.Status.InitContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			return cs.State.Waiting.Reason
		}
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			return cs.State.Waiting.Reason
		}
	}
	return ""
}

func podFlowSignature(flow PodFlow) string {
	return flow.Phase + "|" + flow.Reason + "|" + flow.Node + "|" +
		strconv.FormatInt(int64(flow.Restarts), 10) + "|" +
		strconv.FormatBool(flow.Deleting) + "|" + flow.WorkloadID
}

func shouldEmitFlow(eventType, previous, next string) bool {
	if eventType == string(watch.Added) || eventType == string(watch.Deleted) {
		return true
	}
	return previous != next
}

// GET /api/namespaces/:namespace/flow
// Server-Sent Events. Starts at the current resourceVersion, so existing pods are not replayed as creates.
func NamespaceFlow(c *gin.Context) {
	namespace := c.Param("namespace")
	if !validKubeName(namespace) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid namespace"})
		return
	}
	ctx := c.Request.Context()
	if !namespaceExists(c, ctx, namespace) {
		return
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming is not supported"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	fmt.Fprintf(c.Writer, ": connected\n\n")
	flusher.Flush()

	var (
		mu  sync.RWMutex
		idx ownerIndex
	)
	refresh := func() {
		next, err := loadFlowIndex(ctx, namespace)
		if err != nil {
			return
		}
		mu.Lock()
		idx = next
		mu.Unlock()
	}
	refresh()

	seen := map[string]string{}
	rv := ""
	for ctx.Err() == nil {
		if rv == "" {
			list, err := k8s.Client().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{Limit: 1})
			if err != nil {
				writeFlowProblem(c.Writer, flusher, err)
				return
			}
			rv = list.ResourceVersion
		}
		watcher, err := k8s.Client().CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
			ResourceVersion:     rv,
			AllowWatchBookmarks: true,
		})
		if err != nil {
			if apierrors.IsResourceExpired(err) {
				rv = ""
				continue
			}
			writeFlowProblem(c.Writer, flusher, err)
			return
		}
		err = readPodWatch(ctx, c.Writer, flusher, watcher, &mu, &idx, refresh, seen, &rv)
		watcher.Stop()
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			return
		}
		if errors.Is(err, errWatchExpired) || apierrors.IsResourceExpired(err) {
			rv = ""
			continue
		}
		if err != nil {
			writeFlowProblem(c.Writer, flusher, err)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func loadFlowIndex(ctx context.Context, namespace string) (ownerIndex, error) {
	cs := k8s.Client()
	var replicaSets []appsv1.ReplicaSet
	var jobs []batchv1.Job
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		v, err := cs.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		replicaSets = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		jobs = v.Items
		return nil
	})
	if err := g.Wait(); err != nil {
		return ownerIndex{}, err
	}
	return buildOwnerIndex(nsObjects{ReplicaSets: replicaSets, Jobs: jobs}), nil
}

func readPodWatch(
	ctx context.Context,
	w http.ResponseWriter,
	flusher http.Flusher,
	watcher watch.Interface,
	mu *sync.RWMutex,
	idx *ownerIndex,
	refresh func(),
	seen map[string]string,
	rv *string,
) error {
	refreshTicker := time.NewTicker(30 * time.Second)
	defer refreshTicker.Stop()
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-refreshTicker.C:
			refresh()
		case <-ping.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case ev, ok := <-watcher.ResultChan():
			if !ok {
				return nil
			}
			if obj, ok := ev.Object.(metav1.Object); ok {
				if version := obj.GetResourceVersion(); version != "" {
					*rv = version
				}
			}
			switch ev.Type {
			case watch.Bookmark:
				continue
			case watch.Error:
				status, _ := ev.Object.(*metav1.Status)
				if status != nil && (status.Reason == metav1.StatusReasonExpired || status.Code == http.StatusGone) {
					return errWatchExpired
				}
				if status != nil && status.Message != "" {
					return errors.New(status.Message)
				}
				return errors.New("pod watch failed")
			case watch.Added, watch.Modified, watch.Deleted:
				pod, ok := ev.Object.(*corev1.Pod)
				if !ok || pod == nil {
					continue
				}
				mu.RLock()
				current := *idx
				mu.RUnlock()
				flow := DescribePodFlow(string(ev.Type), pod, current)
				next := podFlowSignature(flow)
				if !shouldEmitFlow(string(ev.Type), seen[pod.Name], next) {
					continue
				}
				if ev.Type == watch.Deleted {
					delete(seen, pod.Name)
				} else {
					seen[pod.Name] = next
				}
				flow.Time = time.Now().UTC()
				payload, err := json.Marshal(flow)
				if err != nil {
					return err
				}
				fmt.Fprintf(w, "event: flow\ndata: %s\n\n", payload)
				flusher.Flush()
			}
		}
	}
}

func writeFlowProblem(w http.ResponseWriter, flusher http.Flusher, err error) {
	payload, marshalErr := json.Marshal(gin.H{"error": err.Error()})
	if marshalErr != nil {
		return
	}
	fmt.Fprintf(w, "event: problem\ndata: %s\n\n", payload)
	flusher.Flush()
}
