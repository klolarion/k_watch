package handler

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"github.com/klolarion/k_watch/backend/k8s"
)

// GET /api/cluster/overview
func ClusterOverview(c *gin.Context) {
	cs := k8s.Client()

	// 리소스 수집용 컨텍스트 (더 긴 타임아웃)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var (
		ns       *corev1.NamespaceList
		node     *corev1.NodeList
		pod      *corev1.PodList
		rs       *appsv1.ReplicaSetList
		dep      *appsv1.DeploymentList
		svc      *corev1.ServiceList
		ing      *networkingv1.IngressList
		job      *batchv1.JobList
		cronJobs *batchv1.CronJobList
		sts      *appsv1.StatefulSetList
		ds       *appsv1.DaemonSetList
		pv       *corev1.PersistentVolumeList
		pvc      *corev1.PersistentVolumeClaimList
	)

	// 병렬 수집 (전체 네임스페이스: "")
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		v, e := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if e == nil {
			ns = v
		}
		return e
	})
	g.Go(func() error {
		v, e := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if e == nil {
			node = v
		}
		return e
	})
	g.Go(func() error {
		v, e := cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		if e == nil {
			pod = v
		}
		return e
	})
	g.Go(func() error {
		v, e := cs.AppsV1().ReplicaSets("").List(ctx, metav1.ListOptions{})
		if e == nil {
			rs = v
		}
		return e
	})
	g.Go(func() error {
		v, e := cs.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
		if e == nil {
			dep = v
		}
		return e
	})
	g.Go(func() error {
		v, e := cs.CoreV1().Services("").List(ctx, metav1.ListOptions{})
		if e == nil {
			svc = v
		}
		return e
	})
	g.Go(func() error {
		v, e := cs.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
		if e == nil {
			ing = v
		}
		return e
	})
	g.Go(func() error {
		v, e := cs.BatchV1().Jobs("").List(ctx, metav1.ListOptions{})
		if e == nil {
			job = v
		}
		return e
	})
	g.Go(func() error {
		v, e := cs.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{})
		if e == nil {
			cronJobs = v
		}
		return e
	})
	g.Go(func() error {
		v, e := cs.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
		if e == nil {
			sts = v
		}
		return e
	})
	g.Go(func() error {
		v, e := cs.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
		if e == nil {
			ds = v
		}
		return e
	})
	g.Go(func() error {
		v, e := cs.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
		if e == nil {
			pv = v
		}
		return e
	})
	g.Go(func() error {
		v, e := cs.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
		if e == nil {
			pvc = v
		}
		return e
	})

	if err := g.Wait(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 메트릭 수집용 별도 컨텍스트 (8초 타임아웃)
	mctx, mcancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer mcancel()

	// 이벤트 수집용 별도 컨텍스트 (5초 타임아웃)
	ectx, ecancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer ecancel()

	// 노드 메트릭을 한 번에 List로 가져오기
	metricsClient := k8s.MetricsClient()
	var nodeMetricsList *metricsv1beta1.NodeMetricsList
	if metricsClient != nil {
		nml, err := metricsClient.MetricsV1beta1().NodeMetricses().List(mctx, metav1.ListOptions{})
		if err == nil {
			nodeMetricsList = nml
		}
	}

	// 이름→메트릭 매핑
	nm := map[string]metricsv1beta1.NodeMetrics{}
	if nodeMetricsList != nil {
		for _, m := range nodeMetricsList.Items {
			nm[m.Name] = m
		}
	}

	// 최근 이벤트 50개. API의 Limit는 최신순이 아니므로 전부 받아서 시간순으로 자른다.
	// ResourceVersion "0"은 etcd 대신 apiserver 캐시에서 읽어서, 이벤트가 많아도 부담이 적다.
	var events []gin.H
	eventList, err := cs.CoreV1().Events("").List(ectx, metav1.ListOptions{ResourceVersion: "0"})
	if err == nil && eventList != nil {
		items := eventList.Items
		sort.Slice(items, func(i, j int) bool {
			return eventTime(items[i]).After(eventTime(items[j]))
		})
		if len(items) > 50 {
			items = items[:50]
		}
		for _, event := range items {
			events = append(events, gin.H{
				"time":      eventTime(event).UTC(),
				"event":     event.Reason,
				"type":      event.Type,
				"namespace": event.Namespace,
				"object":    event.InvolvedObject.Name,
				"message":   event.Message,
			})
		}
	}

	// 네임스페이스별 워크로드 분포 계산
	var workloadDistribution []gin.H
	nsPodCount := make(map[string]int)
	nsServiceCount := make(map[string]int)

	// 파드 수 계산
	for _, p := range pod.Items {
		nsPodCount[p.Namespace]++
	}

	// 서비스 수 계산
	for _, s := range svc.Items {
		nsServiceCount[s.Namespace]++
	}

	// 모든 네임스페이스에 대해 데이터 구성
	for _, ns := range ns.Items {
		podCount := nsPodCount[ns.Name]
		serviceCount := nsServiceCount[ns.Name]

		// 파드와 서비스가 모두 0인 경우 제외
		if podCount > 0 || serviceCount > 0 {
			workloadDistribution = append(workloadDistribution, gin.H{
				"name":     ns.Name,
				"pods":     podCount,
				"services": serviceCount,
			})
		}
	}

	// 노드 상태 구성
	var nodeStatus []gin.H
	for _, n := range node.Items {
		status := "Unknown"
		for _, condition := range n.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				if condition.Status == corev1.ConditionTrue {
					status = "Ready"
				} else {
					status = "NotReady"
				}
				break
			}
		}

		// 노드에 올라간 pod 수 계산
		podCount := 0
		for _, p := range pod.Items {
			if p.Spec.NodeName == n.Name {
				podCount++
			}
		}

		entry := gin.H{
			"node":   n.Name,
			"status": status,
			"pods":   podCount,
		}
		// 사용률은 metrics-server가 있을 때만 싣는다. 없으면 화면이 0%가 아니라 "no metrics"로 그린다.
		if m, ok := nm[n.Name]; ok {
			if cpu := n.Status.Capacity[corev1.ResourceCPU]; cpu.MilliValue() > 0 {
				entry["cpu"] = int(float64(m.Usage.Cpu().MilliValue()) / float64(cpu.MilliValue()) * 100)
			}
			if mem := n.Status.Capacity[corev1.ResourceMemory]; mem.Value() > 0 {
				entry["memory"] = int(float64(m.Usage.Memory().Value()) / float64(mem.Value()) * 100)
			}
		}

		nodeStatus = append(nodeStatus, entry)
	}

	// 오버뷰 응답 구성
	response := gin.H{
		"serverTime": time.Now().UTC(),
		"info": gin.H{
			"namespace":   len(ns.Items),
			"node":        len(node.Items),
			"pod":         len(pod.Items),
			"deployment":  len(dep.Items),
			"service":     len(svc.Items),
			"ingress":     len(ing.Items),
			"replicaSet":  len(rs.Items),
			"statefulSet": len(sts.Items),
			"daemonSet":   len(ds.Items),
			"job":         len(job.Items),
			"cronJob":     len(cronJobs.Items),
			"pv":          len(pv.Items),
			"pvc":         len(pvc.Items),
		},
		"nodeStatus": nodeStatus,
		// 파드 상태 분포 계산
		"podStatus": func() []gin.H {
			statusCount := make(map[string]int)
			for _, pod := range pod.Items {
				status := string(pod.Status.Phase)
				statusCount[status]++
			}

			var result []gin.H
			for status, count := range statusCount {
				if count > 0 { // 0개인 상태는 제외
					result = append(result, gin.H{
						"name":  status,
						"value": count,
					})
				}
			}
			return result
		}(),
		// 워크로드 분포 데이터
		"workloadDistribution": workloadDistribution,
		// 이벤트 타임라인 (최신순 50개)
		"eventTimeline": events,
	}

	c.JSON(http.StatusOK, response)
}

// eventTime picks the most recent timestamp an event carries. Newer events may only set EventTime.
func eventTime(event corev1.Event) time.Time {
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	if !event.EventTime.IsZero() {
		return event.EventTime.Time
	}
	return event.CreationTimestamp.Time
}
