package handler

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/klolarion/k_watch/backend/k8s"
)

func ListNamespaces(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	objects, err := loadCluster(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	summaries := SummarizeNamespaces(
		objects.namespaces,
		objects.pods,
		objects.deployments,
		objects.statefulSets,
		objects.daemonSets,
		objects.jobs,
		objects.cronJobs,
		objects.services,
		objects.ingresses,
		objects.pvcs,
	)
	if summaries == nil {
		summaries = []NamespaceSummary{}
	}
	c.JSON(http.StatusOK, NamespaceListResponse{
		ServerTime: time.Now().UTC(),
		Namespaces: summaries,
	})
}

func NamespaceMap(c *gin.Context) {
	namespace := c.Param("namespace")
	if !validKubeName(namespace) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid namespace"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	if !namespaceExists(c, ctx, namespace) {
		return
	}
	objects, err := loadNamespace(ctx, namespace, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := BuildNamespaceMap(namespace, objects)
	resp.ServerTime = time.Now().UTC()
	c.JSON(http.StatusOK, resp)
}

func WorkloadFocus(c *gin.Context) {
	namespace := c.Param("namespace")
	kind, ok := canonicalKind(c.Param("kind"))
	name := c.Param("name")
	if !validKubeName(namespace) || !validKubeName(name) || !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workload"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	if !namespaceExists(c, ctx, namespace) {
		return
	}
	objects, err := loadNamespace(ctx, namespace, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp, err := BuildFocus(namespace, kind, name, objects)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "workload not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp.ServerTime = time.Now().UTC()
	c.JSON(http.StatusOK, resp)
}

func namespaceExists(c *gin.Context, ctx context.Context, namespace string) bool {
	_, err := k8s.Client().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		return true
	}
	if apierrors.IsNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "namespace not found"})
		return false
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	return false
}

func validKubeName(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}
	if name[0] == '-' || name[0] == '.' || name[len(name)-1] == '-' || name[len(name)-1] == '.' {
		return false
	}
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '.' {
			continue
		}
		return false
	}
	return true
}

type clusterObjects struct {
	namespaces   []corev1.Namespace
	pods         []corev1.Pod
	deployments  []appsv1.Deployment
	statefulSets []appsv1.StatefulSet
	daemonSets   []appsv1.DaemonSet
	jobs         []batchv1.Job
	cronJobs     []batchv1.CronJob
	services     []corev1.Service
	ingresses    []networkingv1.Ingress
	pvcs         []corev1.PersistentVolumeClaim
}

func loadCluster(ctx context.Context) (clusterObjects, error) {
	cs := k8s.Client()
	var out clusterObjects
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		v, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.namespaces = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.pods = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.deployments = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.statefulSets = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.daemonSets = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.BatchV1().Jobs("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.jobs = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.cronJobs = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.CoreV1().Services("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.services = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.ingresses = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.pvcs = v.Items
		return nil
	})
	return out, g.Wait()
}

func loadNamespace(ctx context.Context, namespace string, endpoints bool) (nsObjects, error) {
	cs := k8s.Client()
	var out nsObjects
	// gctx is cancelled once Wait returns, so the PV lookup below must use ctx, not gctx.
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		v, err := cs.CoreV1().Pods(namespace).List(gctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.Pods = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.AppsV1().ReplicaSets(namespace).List(gctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.ReplicaSets = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.AppsV1().Deployments(namespace).List(gctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.Deployments = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.AppsV1().StatefulSets(namespace).List(gctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.StatefulSets = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.AppsV1().DaemonSets(namespace).List(gctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.DaemonSets = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.BatchV1().Jobs(namespace).List(gctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.Jobs = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.BatchV1().CronJobs(namespace).List(gctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.CronJobs = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.CoreV1().Services(namespace).List(gctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.Services = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.NetworkingV1().Ingresses(namespace).List(gctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.Ingresses = v.Items
		return nil
	})
	g.Go(func() error {
		v, err := cs.CoreV1().PersistentVolumeClaims(namespace).List(gctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		out.PVCs = v.Items
		return nil
	})
	if endpoints {
		g.Go(func() error {
			v, err := cs.DiscoveryV1().EndpointSlices(namespace).List(gctx, metav1.ListOptions{})
			if err != nil {
				return err
			}
			out.EndpointSlices = v.Items
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return out, err
	}
	pvs, err := loadBoundVolumes(ctx, cs, out.PVCs)
	if err != nil {
		return out, err
	}
	out.PVs = pvs
	return out, nil
}

func loadBoundVolumes(ctx context.Context, cs *kubernetes.Clientset, pvcs []corev1.PersistentVolumeClaim) ([]corev1.PersistentVolume, error) {
	names := map[string]struct{}{}
	for _, pvc := range pvcs {
		if pvc.Spec.VolumeName != "" {
			names[pvc.Spec.VolumeName] = struct{}{}
		}
	}
	if len(names) == 0 {
		return nil, nil
	}
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(8)
	var mu sync.Mutex
	var pvs []corev1.PersistentVolume
	for name := range names {
		name := name
		g.Go(func() error {
			pv, err := cs.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
			mu.Lock()
			pvs = append(pvs, *pv)
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return pvs, nil
}
