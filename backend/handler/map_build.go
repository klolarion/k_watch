package handler

import (
	"errors"
	"fmt"
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// 완료된 단독 Job은 지도에 최신 N개만 올린다. CronJob이 만든 완료 Job은 올리지 않는다.
	finishedJobLimit = 20
	// 포커스 목록에 개별 Pod를 전부 넣지 않는다.
	focusPodLimit = 50
)

var ErrNotFound = errors.New("not found")

type PodCounts struct {
	Total     int `json:"total"`
	Running   int `json:"running"`
	Pending   int `json:"pending"`
	Failed    int `json:"failed"`
	Succeeded int `json:"succeeded"`
	Unknown   int `json:"unknown"`
}

func (p *PodCounts) add(phase corev1.PodPhase) {
	p.Total++
	switch phase {
	case corev1.PodRunning:
		p.Running++
	case corev1.PodPending:
		p.Pending++
	case corev1.PodFailed:
		p.Failed++
	case corev1.PodSucceeded:
		p.Succeeded++
	default:
		p.Unknown++
	}
}

type MapNode struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Label     string     `json:"label"`
	Namespace string     `json:"namespace,omitempty"`
	Replicas  *int32     `json:"replicas,omitempty"`
	Available *int32     `json:"available,omitempty"`
	Pods      *PodCounts `json:"pods,omitempty"`
	Status    string     `json:"status,omitempty"`
}

type MapEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type MapResponse struct {
	ServerTime       time.Time `json:"serverTime"`
	Namespace        string    `json:"namespace"`
	Nodes            []MapNode `json:"nodes"`
	Edges            []MapEdge `json:"edges"`
	HiddenJobs       int       `json:"hiddenJobs"`
	FinishedJobLimit int       `json:"finishedJobLimit"`
}

type NamespaceSummary struct {
	Name          string `json:"name"`
	Pods          int    `json:"pods"`
	UnhealthyPods int    `json:"unhealthyPods"`
	Deployments   int    `json:"deployments"`
	StatefulSets  int    `json:"statefulSets"`
	DaemonSets    int    `json:"daemonSets"`
	Jobs          int    `json:"jobs"`
	CronJobs      int    `json:"cronJobs"`
	Services      int    `json:"services"`
	Ingresses     int    `json:"ingresses"`
	PVCs          int    `json:"pvcs"`
}

type NamespaceListResponse struct {
	ServerTime time.Time          `json:"serverTime"`
	Namespaces []NamespaceSummary `json:"namespaces"`
}

type PodRow struct {
	Name     string `json:"name"`
	Phase    string `json:"phase"`
	Node     string `json:"node"`
	Restarts int32  `json:"restarts"`
}

type ServiceRef struct {
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Ports []string `json:"ports"`
}

type IngressRef struct {
	Name  string   `json:"name"`
	Hosts []string `json:"hosts"`
}

type NodePlacement struct {
	Name    string `json:"name"`
	Pods    int    `json:"pods"`
	Running int    `json:"running"`
}

type VolumeRef struct {
	Name      string   `json:"name"`
	Status    string   `json:"status,omitempty"`
	Size      string   `json:"size,omitempty"`
	Workloads []string `json:"workloads"`
}

type FocusResponse struct {
	ServerTime    time.Time       `json:"serverTime"`
	Target        MapNode         `json:"target"`
	Pods          []PodRow        `json:"pods"`
	PodsTruncated bool            `json:"podsTruncated"`
	PodTotal      int             `json:"podTotal"`
	PodLimit      int             `json:"podLimit"`
	Services      []ServiceRef    `json:"services"`
	Ingresses     []IngressRef    `json:"ingresses"`
	Nodes         []NodePlacement `json:"nodes"`
	Routes        []MapEdge       `json:"routes"`
	Highlight     []string        `json:"highlight"`
	Volume        *VolumeRef      `json:"volume,omitempty"`
}

type nsObjects struct {
	Pods           []corev1.Pod
	ReplicaSets    []appsv1.ReplicaSet
	Deployments    []appsv1.Deployment
	StatefulSets   []appsv1.StatefulSet
	DaemonSets     []appsv1.DaemonSet
	Jobs           []batchv1.Job
	CronJobs       []batchv1.CronJob
	Services       []corev1.Service
	Ingresses      []networkingv1.Ingress
	PVCs           []corev1.PersistentVolumeClaim
	PVs            []corev1.PersistentVolume
	EndpointSlices []discoveryv1.EndpointSlice
}

type workloadRef struct {
	Kind string
	Name string
}

type ownerIndex struct {
	rsDeploy map[string]string
	jobCron  map[string]string
}

func buildOwnerIndex(o nsObjects) ownerIndex {
	idx := ownerIndex{
		rsDeploy: map[string]string{},
		jobCron:  map[string]string{},
	}
	for i := range o.ReplicaSets {
		rs := &o.ReplicaSets[i]
		if name, ok := controllerName(rs.OwnerReferences, "Deployment"); ok {
			idx.rsDeploy[rs.Name] = name
		}
	}
	for i := range o.Jobs {
		job := &o.Jobs[i]
		if name, ok := controllerName(job.OwnerReferences, "CronJob"); ok {
			idx.jobCron[job.Name] = name
		}
	}
	return idx
}

func controllerName(refs []metav1.OwnerReference, kind string) (string, bool) {
	for _, ref := range refs {
		if ref.Kind != kind {
			continue
		}
		if ref.Controller != nil && !*ref.Controller {
			continue
		}
		return ref.Name, true
	}
	return "", false
}

func controllerRef(refs []metav1.OwnerReference) (metav1.OwnerReference, bool) {
	for _, ref := range refs {
		if ref.Controller != nil && *ref.Controller {
			return ref, true
		}
	}
	if len(refs) > 0 {
		return refs[0], true
	}
	return metav1.OwnerReference{}, false
}

func resolveWorkload(pod corev1.Pod, idx ownerIndex) (workloadRef, bool) {
	owner, ok := controllerRef(pod.OwnerReferences)
	if !ok {
		return workloadRef{}, false
	}
	switch owner.Kind {
	case "ReplicaSet":
		if deploy, ok := idx.rsDeploy[owner.Name]; ok {
			return workloadRef{Kind: "Deployment", Name: deploy}, true
		}
		return workloadRef{}, false
	case "StatefulSet":
		return workloadRef{Kind: "StatefulSet", Name: owner.Name}, true
	case "DaemonSet":
		return workloadRef{Kind: "DaemonSet", Name: owner.Name}, true
	case "Job":
		if cron, ok := idx.jobCron[owner.Name]; ok {
			return workloadRef{Kind: "CronJob", Name: cron}, true
		}
		return workloadRef{Kind: "Job", Name: owner.Name}, true
	default:
		return workloadRef{}, false
	}
}

func podUnhealthy(pod corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodSucceeded {
		return false
	}
	if pod.Status.Phase != corev1.PodRunning {
		return true
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			return true
		}
	}
	return false
}

func jobFinished(job batchv1.Job) bool {
	for _, cond := range job.Status.Conditions {
		if (cond.Type == batchv1.JobComplete || cond.Type == batchv1.JobFailed) && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func nodeID(kind, namespace, name string) string {
	switch kind {
	case "Namespace":
		return "ns:" + name
	case "PersistentVolume":
		return "pv:" + name
	case "Deployment":
		return "deploy:" + namespace + "/" + name
	case "StatefulSet":
		return "sts:" + namespace + "/" + name
	case "DaemonSet":
		return "ds:" + namespace + "/" + name
	case "Job":
		return "job:" + namespace + "/" + name
	case "CronJob":
		return "cronjob:" + namespace + "/" + name
	case "Service":
		return "svc:" + namespace + "/" + name
	case "Ingress":
		return "ing:" + namespace + "/" + name
	case "PersistentVolumeClaim":
		return "pvc:" + namespace + "/" + name
	case "PodGroup":
		return "podgroup:" + namespace + "/" + name
	default:
		return kind + ":" + namespace + "/" + name
	}
}

func int32ptr(v int32) *int32 { return &v }

func SummarizeNamespaces(
	namespaces []corev1.Namespace,
	pods []corev1.Pod,
	deployments []appsv1.Deployment,
	statefulSets []appsv1.StatefulSet,
	daemonSets []appsv1.DaemonSet,
	jobs []batchv1.Job,
	cronJobs []batchv1.CronJob,
	services []corev1.Service,
	ingresses []networkingv1.Ingress,
	pvcs []corev1.PersistentVolumeClaim,
) []NamespaceSummary {
	byName := map[string]*NamespaceSummary{}
	order := make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		if _, ok := byName[ns.Name]; ok {
			continue
		}
		byName[ns.Name] = &NamespaceSummary{Name: ns.Name}
		order = append(order, ns.Name)
	}
	ensure := func(ns string) *NamespaceSummary {
		if s, ok := byName[ns]; ok {
			return s
		}
		s := &NamespaceSummary{Name: ns}
		byName[ns] = s
		order = append(order, ns)
		return s
	}
	for _, pod := range pods {
		s := ensure(pod.Namespace)
		s.Pods++
		if podUnhealthy(pod) {
			s.UnhealthyPods++
		}
	}
	for _, item := range deployments {
		ensure(item.Namespace).Deployments++
	}
	for _, item := range statefulSets {
		ensure(item.Namespace).StatefulSets++
	}
	for _, item := range daemonSets {
		ensure(item.Namespace).DaemonSets++
	}
	for _, item := range jobs {
		ensure(item.Namespace).Jobs++
	}
	for _, item := range cronJobs {
		ensure(item.Namespace).CronJobs++
	}
	for _, item := range services {
		ensure(item.Namespace).Services++
	}
	for _, item := range ingresses {
		ensure(item.Namespace).Ingresses++
	}
	for _, item := range pvcs {
		ensure(item.Namespace).PVCs++
	}

	sort.Slice(order, func(i, j int) bool {
		a, b := byName[order[i]], byName[order[j]]
		if a.Pods != b.Pods {
			return a.Pods > b.Pods
		}
		return a.Name < b.Name
	})
	out := make([]NamespaceSummary, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out
}

type shownJobs struct {
	underCron  []batchv1.Job
	standalone []batchv1.Job
	hidden     int
}

func selectJobs(jobs []batchv1.Job, idx ownerIndex) shownJobs {
	var underCron []batchv1.Job
	var activeStandalone []batchv1.Job
	var finishedStandalone []batchv1.Job
	hidden := 0
	for _, job := range jobs {
		if _, owned := idx.jobCron[job.Name]; owned {
			if jobFinished(job) {
				hidden++
				continue
			}
			underCron = append(underCron, job)
			continue
		}
		if jobFinished(job) {
			finishedStandalone = append(finishedStandalone, job)
			continue
		}
		activeStandalone = append(activeStandalone, job)
	}
	sort.Slice(finishedStandalone, func(i, j int) bool {
		return finishedStandalone[i].CreationTimestamp.After(finishedStandalone[j].CreationTimestamp.Time)
	})
	if len(finishedStandalone) > finishedJobLimit {
		hidden += len(finishedStandalone) - finishedJobLimit
		finishedStandalone = finishedStandalone[:finishedJobLimit]
	}
	standalone := append(activeStandalone, finishedStandalone...)
	return shownJobs{underCron: underCron, standalone: standalone, hidden: hidden}
}

func BuildNamespaceMap(namespace string, o nsObjects) MapResponse {
	idx := buildOwnerIndex(o)
	jobs := selectJobs(o.Jobs, idx)
	podCounts := map[workloadRef]*PodCounts{}
	var unmanaged PodCounts
	addCount := func(ref workloadRef, phase corev1.PodPhase) {
		counts := podCounts[ref]
		if counts == nil {
			counts = &PodCounts{}
			podCounts[ref] = counts
		}
		counts.add(phase)
	}
	for _, pod := range o.Pods {
		if pod.Namespace != namespace {
			continue
		}
		owner, hasOwner := controllerRef(pod.OwnerReferences)
		if hasOwner && owner.Kind == "Job" {
			addCount(workloadRef{Kind: "Job", Name: owner.Name}, pod.Status.Phase)
			if cron, ok := idx.jobCron[owner.Name]; ok {
				addCount(workloadRef{Kind: "CronJob", Name: cron}, pod.Status.Phase)
			}
			continue
		}
		ref, ok := resolveWorkload(pod, idx)
		if !ok {
			unmanaged.add(pod.Status.Phase)
			continue
		}
		addCount(ref, pod.Status.Phase)
	}

	nodes := make([]MapNode, 0)
	edges := make([]MapEdge, 0)
	nsID := nodeID("Namespace", namespace, namespace)
	nodes = append(nodes, MapNode{ID: nsID, Type: "Namespace", Label: namespace, Namespace: namespace})

	addContains := func(node MapNode) {
		nodes = append(nodes, node)
		edges = append(edges, MapEdge{From: nsID, To: node.ID, Kind: "contains"})
	}

	for i := range o.Deployments {
		item := &o.Deployments[i]
		if item.Namespace != namespace {
			continue
		}
		replicas := int32(1)
		if item.Spec.Replicas != nil {
			replicas = *item.Spec.Replicas
		}
		addContains(MapNode{
			ID:        nodeID("Deployment", namespace, item.Name),
			Type:      "Deployment",
			Label:     item.Name,
			Namespace: namespace,
			Replicas:  int32ptr(replicas),
			Available: int32ptr(item.Status.AvailableReplicas),
			Pods:      countsOrEmpty(podCounts[workloadRef{Kind: "Deployment", Name: item.Name}]),
		})
	}
	for i := range o.StatefulSets {
		item := &o.StatefulSets[i]
		if item.Namespace != namespace {
			continue
		}
		replicas := int32(1)
		if item.Spec.Replicas != nil {
			replicas = *item.Spec.Replicas
		}
		addContains(MapNode{
			ID:        nodeID("StatefulSet", namespace, item.Name),
			Type:      "StatefulSet",
			Label:     item.Name,
			Namespace: namespace,
			Replicas:  int32ptr(replicas),
			Available: int32ptr(item.Status.ReadyReplicas),
			Pods:      countsOrEmpty(podCounts[workloadRef{Kind: "StatefulSet", Name: item.Name}]),
		})
	}
	for i := range o.DaemonSets {
		item := &o.DaemonSets[i]
		if item.Namespace != namespace {
			continue
		}
		addContains(MapNode{
			ID:        nodeID("DaemonSet", namespace, item.Name),
			Type:      "DaemonSet",
			Label:     item.Name,
			Namespace: namespace,
			Replicas:  int32ptr(item.Status.DesiredNumberScheduled),
			Available: int32ptr(item.Status.NumberAvailable),
			Pods:      countsOrEmpty(podCounts[workloadRef{Kind: "DaemonSet", Name: item.Name}]),
		})
	}
	for i := range o.CronJobs {
		item := &o.CronJobs[i]
		if item.Namespace != namespace {
			continue
		}
		addContains(MapNode{
			ID:        nodeID("CronJob", namespace, item.Name),
			Type:      "CronJob",
			Label:     item.Name,
			Namespace: namespace,
			Pods:      countsOrEmpty(podCounts[workloadRef{Kind: "CronJob", Name: item.Name}]),
		})
	}
	for _, job := range jobs.standalone {
		if job.Namespace != namespace {
			continue
		}
		addContains(jobNode(namespace, job, podCounts))
	}
	for _, job := range jobs.underCron {
		if job.Namespace != namespace {
			continue
		}
		node := jobNode(namespace, job, podCounts)
		nodes = append(nodes, node)
		cronName, _ := controllerName(job.OwnerReferences, "CronJob")
		edges = append(edges, MapEdge{
			From: nodeID("CronJob", namespace, cronName),
			To:   node.ID,
			Kind: "owns",
		})
	}
	for i := range o.Services {
		item := &o.Services[i]
		if item.Namespace != namespace {
			continue
		}
		addContains(MapNode{
			ID:        nodeID("Service", namespace, item.Name),
			Type:      "Service",
			Label:     item.Name,
			Namespace: namespace,
			Status:    string(item.Spec.Type),
		})
	}
	for i := range o.Ingresses {
		item := &o.Ingresses[i]
		if item.Namespace != namespace {
			continue
		}
		addContains(MapNode{
			ID:        nodeID("Ingress", namespace, item.Name),
			Type:      "Ingress",
			Label:     item.Name,
			Namespace: namespace,
		})
	}

	pvByName := map[string]corev1.PersistentVolume{}
	for _, pv := range o.PVs {
		pvByName[pv.Name] = pv
	}
	for i := range o.PVCs {
		item := &o.PVCs[i]
		if item.Namespace != namespace {
			continue
		}
		pvcNode := MapNode{
			ID:        nodeID("PersistentVolumeClaim", namespace, item.Name),
			Type:      "PersistentVolumeClaim",
			Label:     item.Name,
			Namespace: namespace,
			Status:    string(item.Status.Phase),
		}
		addContains(pvcNode)
		if item.Spec.VolumeName == "" {
			continue
		}
		pv, ok := pvByName[item.Spec.VolumeName]
		if !ok {
			continue
		}
		pvID := nodeID("PersistentVolume", "", pv.Name)
		if !hasNode(nodes, pvID) {
			size := ""
			if q, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
				size = q.String()
			}
			nodes = append(nodes, MapNode{
				ID:     pvID,
				Type:   "PersistentVolume",
				Label:  pv.Name,
				Status: string(pv.Status.Phase),
				Pods:   nil,
			})
			if size != "" {
				nodes[len(nodes)-1].Status = string(pv.Status.Phase) + " " + size
			}
		}
		edges = append(edges, MapEdge{From: pvID, To: pvcNode.ID, Kind: "binds"})
	}

	if unmanaged.Total > 0 {
		addContains(MapNode{
			ID:        nodeID("PodGroup", namespace, "unmanaged"),
			Type:      "PodGroup",
			Label:     "Unmanaged pods",
			Namespace: namespace,
			Pods:      &unmanaged,
		})
	}

	sortNodes(nodes)
	sortEdges(edges)
	return MapResponse{
		Namespace:        namespace,
		Nodes:            nodes,
		Edges:            edges,
		HiddenJobs:       jobs.hidden,
		FinishedJobLimit: finishedJobLimit,
	}
}

func jobNode(namespace string, job batchv1.Job, podCounts map[workloadRef]*PodCounts) MapNode {
	parallelism := int32(1)
	if job.Spec.Parallelism != nil {
		parallelism = *job.Spec.Parallelism
	}
	succeeded := job.Status.Succeeded
	return MapNode{
		ID:        nodeID("Job", namespace, job.Name),
		Type:      "Job",
		Label:     job.Name,
		Namespace: namespace,
		Replicas:  int32ptr(parallelism),
		Available: int32ptr(succeeded),
		Pods:      countsOrEmpty(podCounts[workloadRef{Kind: "Job", Name: job.Name}]),
	}
}

func countsOrEmpty(counts *PodCounts) *PodCounts {
	if counts == nil {
		return &PodCounts{}
	}
	copy := *counts
	return &copy
}

func hasNode(nodes []MapNode, id string) bool {
	for _, node := range nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func sortNodes(nodes []MapNode) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
}

func sortEdges(edges []MapEdge) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Kind < edges[j].Kind
	})
}

func canonicalKind(urlKind string) (string, bool) {
	switch urlKind {
	case "deployment":
		return "Deployment", true
	case "statefulset":
		return "StatefulSet", true
	case "daemonset":
		return "DaemonSet", true
	case "job":
		return "Job", true
	case "cronjob":
		return "CronJob", true
	case "service":
		return "Service", true
	case "ingress":
		return "Ingress", true
	case "pvc", "persistentvolumeclaim":
		return "PersistentVolumeClaim", true
	case "podgroup":
		return "PodGroup", true
	default:
		return "", false
	}
}

func BuildFocus(namespace, kind, name string, o nsObjects) (FocusResponse, error) {
	idx := buildOwnerIndex(o)
	member, target, err := selectFocus(namespace, kind, name, o, idx)
	if err != nil {
		return FocusResponse{}, err
	}

	serviceNames := map[string]struct{}{}
	switch kind {
	case "Service":
		serviceNames[name] = struct{}{}
	case "Ingress":
		for _, svc := range servicesForIngress(findIngress(o.Ingresses, namespace, name)) {
			serviceNames[svc] = struct{}{}
		}
	default:
		for _, slice := range o.EndpointSlices {
			if slice.Namespace != namespace {
				continue
			}
			svc := slice.Labels["kubernetes.io/service-name"]
			if svc == "" {
				continue
			}
			for _, ep := range slice.Endpoints {
				if ep.TargetRef == nil || ep.TargetRef.Kind != "Pod" {
					continue
				}
				if _, ok := member[ep.TargetRef.Name]; ok {
					serviceNames[svc] = struct{}{}
				}
			}
		}
	}
	if kind == "Service" {
		member = podsForServices(namespace, serviceNames, o, member)
	}

	var services []ServiceRef
	for _, svc := range o.Services {
		if svc.Namespace != namespace {
			continue
		}
		if _, ok := serviceNames[svc.Name]; !ok {
			continue
		}
		services = append(services, serviceRef(svc))
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })

	var ingresses []IngressRef
	for _, ing := range o.Ingresses {
		if ing.Namespace != namespace {
			continue
		}
		backends := servicesForIngress(&ing)
		hit := false
		for _, svc := range backends {
			if _, ok := serviceNames[svc]; ok {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		ingresses = append(ingresses, ingressRef(ing))
	}
	sort.Slice(ingresses, func(i, j int) bool { return ingresses[i].Name < ingresses[j].Name })

	workloads := map[string]workloadRef{}
	for _, pod := range member {
		if ref, ok := resolveWorkload(pod, idx); ok {
			workloads[nodeID(ref.Kind, namespace, ref.Name)] = ref
		} else {
			workloads[nodeID("PodGroup", namespace, "unmanaged")] = workloadRef{Kind: "PodGroup", Name: "unmanaged"}
		}
	}

	var routes []MapEdge
	addRoute := func(from, to string) {
		if from == "" || to == "" || from == to {
			return
		}
		routes = append(routes, MapEdge{From: from, To: to, Kind: "routes"})
	}
	switch kind {
	case "Ingress":
		ingID := nodeID("Ingress", namespace, name)
		for svc := range serviceNames {
			svcID := nodeID("Service", namespace, svc)
			addRoute(ingID, svcID)
			for id := range workloads {
				addRoute(svcID, id)
			}
		}
	case "Service":
		svcID := nodeID("Service", namespace, name)
		for _, ing := range ingresses {
			addRoute(nodeID("Ingress", namespace, ing.Name), svcID)
		}
		for id := range workloads {
			addRoute(svcID, id)
		}
	case "PersistentVolumeClaim":
		// 스토리지 연결은 binds로 지도에 이미 있다. 호출 경로는 없다.
	default:
		targetID := nodeID(kind, namespace, name)
		for svc := range serviceNames {
			svcID := nodeID("Service", namespace, svc)
			addRoute(svcID, targetID)
			for _, ing := range ingresses {
				addRoute(nodeID("Ingress", namespace, ing.Name), svcID)
			}
		}
	}
	sortEdges(routes)

	highlightSet := map[string]struct{}{
		nodeID("Namespace", namespace, namespace): {},
		target.ID: {},
	}
	for _, svc := range services {
		highlightSet[nodeID("Service", namespace, svc.Name)] = struct{}{}
	}
	for _, ing := range ingresses {
		highlightSet[nodeID("Ingress", namespace, ing.Name)] = struct{}{}
	}
	for id := range workloads {
		highlightSet[id] = struct{}{}
	}

	var volume *VolumeRef
	if kind == "PersistentVolumeClaim" {
		pvc := findPVC(o.PVCs, namespace, name)
		if pvc != nil && pvc.Spec.VolumeName != "" {
			volume = &VolumeRef{Name: pvc.Spec.VolumeName, Workloads: workloadNames(workloads)}
			highlightSet[nodeID("PersistentVolume", "", pvc.Spec.VolumeName)] = struct{}{}
			for _, pv := range o.PVs {
				if pv.Name != pvc.Spec.VolumeName {
					continue
				}
				volume.Status = string(pv.Status.Phase)
				if q, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
					volume.Size = q.String()
				}
			}
		} else if pvc != nil {
			volume = &VolumeRef{Workloads: workloadNames(workloads)}
		}
	}

	rows := make([]PodRow, 0, len(member))
	nodeCounts := map[string]*NodePlacement{}
	for _, pod := range member {
		nodeName := pod.Spec.NodeName
		if nodeName == "" {
			nodeName = "(unscheduled)"
		}
		placement := nodeCounts[nodeName]
		if placement == nil {
			placement = &NodePlacement{Name: nodeName}
			nodeCounts[nodeName] = placement
		}
		placement.Pods++
		if pod.Status.Phase == corev1.PodRunning {
			placement.Running++
		}
		var restarts int32
		for _, cs := range pod.Status.ContainerStatuses {
			restarts += cs.RestartCount
		}
		rows = append(rows, PodRow{
			Name:     pod.Name,
			Phase:    string(pod.Status.Phase),
			Node:     nodeName,
			Restarts: restarts,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		ri, rj := phaseRank(rows[i].Phase), phaseRank(rows[j].Phase)
		if ri != rj {
			return ri < rj
		}
		return rows[i].Name < rows[j].Name
	})
	truncated := false
	if len(rows) > focusPodLimit {
		truncated = true
		rows = rows[:focusPodLimit]
	}
	nodes := make([]NodePlacement, 0, len(nodeCounts))
	for _, placement := range nodeCounts {
		nodes = append(nodes, *placement)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Pods != nodes[j].Pods {
			return nodes[i].Pods > nodes[j].Pods
		}
		return nodes[i].Name < nodes[j].Name
	})
	highlight := make([]string, 0, len(highlightSet))
	for id := range highlightSet {
		highlight = append(highlight, id)
	}
	sort.Strings(highlight)

	counts := PodCounts{}
	for _, pod := range member {
		counts.add(pod.Status.Phase)
	}
	target.Pods = &counts

	if services == nil {
		services = []ServiceRef{}
	}
	if ingresses == nil {
		ingresses = []IngressRef{}
	}
	// The UI reads these as arrays. A workload without a Service has no routes, which must be [] not null.
	if routes == nil {
		routes = []MapEdge{}
	}
	return FocusResponse{
		Target:        target,
		Pods:          rows,
		PodsTruncated: truncated,
		PodTotal:      len(member),
		PodLimit:      focusPodLimit,
		Services:      services,
		Ingresses:     ingresses,
		Nodes:         nodes,
		Routes:        routes,
		Highlight:     highlight,
		Volume:        volume,
	}, nil
}

func podsForServices(namespace string, serviceNames map[string]struct{}, o nsObjects, seed map[string]corev1.Pod) map[string]corev1.Pod {
	names := map[string]struct{}{}
	for _, slice := range o.EndpointSlices {
		if slice.Namespace != namespace {
			continue
		}
		svc := slice.Labels["kubernetes.io/service-name"]
		if _, ok := serviceNames[svc]; !ok {
			continue
		}
		for _, ep := range slice.Endpoints {
			if ep.TargetRef != nil && ep.TargetRef.Kind == "Pod" {
				names[ep.TargetRef.Name] = struct{}{}
			}
		}
	}
	if len(names) == 0 {
		return seed
	}
	out := map[string]corev1.Pod{}
	for _, pod := range o.Pods {
		if pod.Namespace != namespace {
			continue
		}
		if _, ok := names[pod.Name]; ok {
			out[pod.Name] = pod
		}
	}
	return out
}

func selectFocus(namespace, kind, name string, o nsObjects, idx ownerIndex) (map[string]corev1.Pod, MapNode, error) {
	member := map[string]corev1.Pod{}
	switch kind {
	case "Deployment":
		if findDeployment(o.Deployments, namespace, name) == nil {
			return nil, MapNode{}, ErrNotFound
		}
	case "StatefulSet":
		if findStatefulSet(o.StatefulSets, namespace, name) == nil {
			return nil, MapNode{}, ErrNotFound
		}
	case "DaemonSet":
		if findDaemonSet(o.DaemonSets, namespace, name) == nil {
			return nil, MapNode{}, ErrNotFound
		}
	case "Job":
		if findJob(o.Jobs, namespace, name) == nil {
			return nil, MapNode{}, ErrNotFound
		}
	case "CronJob":
		if findCronJob(o.CronJobs, namespace, name) == nil {
			return nil, MapNode{}, ErrNotFound
		}
	case "Service":
		if findService(o.Services, namespace, name) == nil {
			return nil, MapNode{}, ErrNotFound
		}
	case "Ingress":
		if findIngress(o.Ingresses, namespace, name) == nil {
			return nil, MapNode{}, ErrNotFound
		}
	case "PersistentVolumeClaim":
		if findPVC(o.PVCs, namespace, name) == nil {
			return nil, MapNode{}, ErrNotFound
		}
	case "PodGroup":
		if name != "unmanaged" {
			return nil, MapNode{}, ErrNotFound
		}
	default:
		return nil, MapNode{}, ErrNotFound
	}

	target := MapNode{
		ID:        nodeID(kind, namespace, name),
		Type:      kind,
		Label:     name,
		Namespace: namespace,
	}
	if kind == "PodGroup" {
		target.Label = "Unmanaged pods"
	}

	for _, pod := range o.Pods {
		if pod.Namespace != namespace {
			continue
		}
		if podInFocus(pod, namespace, kind, name, o, idx) {
			member[pod.Name] = pod
		}
	}
	counts := PodCounts{}
	for _, pod := range member {
		counts.add(pod.Status.Phase)
	}
	target.Pods = &counts
	return member, target, nil
}

func podInFocus(pod corev1.Pod, namespace, kind, name string, o nsObjects, idx ownerIndex) bool {
	switch kind {
	case "PersistentVolumeClaim":
		for _, vol := range pod.Spec.Volumes {
			if vol.PersistentVolumeClaim != nil && vol.PersistentVolumeClaim.ClaimName == name {
				return true
			}
		}
		return false
	case "Service":
		return false
	case "Ingress":
		ing := findIngress(o.Ingresses, namespace, name)
		if ing == nil {
			return false
		}
		backends := map[string]struct{}{}
		for _, svc := range servicesForIngress(ing) {
			backends[svc] = struct{}{}
		}
		for _, slice := range o.EndpointSlices {
			if slice.Namespace != namespace {
				continue
			}
			if _, ok := backends[slice.Labels["kubernetes.io/service-name"]]; !ok {
				continue
			}
			for _, ep := range slice.Endpoints {
				if ep.TargetRef != nil && ep.TargetRef.Kind == "Pod" && ep.TargetRef.Name == pod.Name {
					return true
				}
			}
		}
		return false
	case "PodGroup":
		_, ok := resolveWorkload(pod, idx)
		return !ok
	case "Job":
		owner, ok := controllerRef(pod.OwnerReferences)
		return ok && owner.Kind == "Job" && owner.Name == name
	default:
		ref, ok := resolveWorkload(pod, idx)
		return ok && ref.Kind == kind && ref.Name == name
	}
}

func workloadNames(workloads map[string]workloadRef) []string {
	names := make([]string, 0, len(workloads))
	for _, ref := range workloads {
		if ref.Kind == "PodGroup" {
			names = append(names, "Unmanaged pods")
			continue
		}
		names = append(names, ref.Kind+"/"+ref.Name)
	}
	sort.Strings(names)
	return names
}

func serviceRef(svc corev1.Service) ServiceRef {
	ports := make([]string, 0, len(svc.Spec.Ports))
	for _, port := range svc.Spec.Ports {
		ports = append(ports, fmt.Sprintf("%d/%s", port.Port, port.Protocol))
	}
	return ServiceRef{Name: svc.Name, Type: string(svc.Spec.Type), Ports: ports}
}

func ingressRef(ing networkingv1.Ingress) IngressRef {
	hosts := make([]string, 0)
	seen := map[string]struct{}{}
	for _, rule := range ing.Spec.Rules {
		if rule.Host == "" {
			continue
		}
		if _, ok := seen[rule.Host]; ok {
			continue
		}
		seen[rule.Host] = struct{}{}
		hosts = append(hosts, rule.Host)
	}
	sort.Strings(hosts)
	return IngressRef{Name: ing.Name, Hosts: hosts}
}

func servicesForIngress(ing *networkingv1.Ingress) []string {
	if ing == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var names []string
	add := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		add(ing.Spec.DefaultBackend.Service.Name)
	}
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil {
				add(path.Backend.Service.Name)
			}
		}
	}
	sort.Strings(names)
	return names
}

func phaseRank(phase string) int {
	switch corev1.PodPhase(phase) {
	case corev1.PodFailed:
		return 0
	case corev1.PodPending:
		return 1
	case corev1.PodUnknown:
		return 2
	case corev1.PodRunning:
		return 3
	case corev1.PodSucceeded:
		return 4
	default:
		return 5
	}
}

func findDeployment(items []appsv1.Deployment, namespace, name string) *appsv1.Deployment {
	for i := range items {
		if items[i].Namespace == namespace && items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}

func findStatefulSet(items []appsv1.StatefulSet, namespace, name string) *appsv1.StatefulSet {
	for i := range items {
		if items[i].Namespace == namespace && items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}

func findDaemonSet(items []appsv1.DaemonSet, namespace, name string) *appsv1.DaemonSet {
	for i := range items {
		if items[i].Namespace == namespace && items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}

func findJob(items []batchv1.Job, namespace, name string) *batchv1.Job {
	for i := range items {
		if items[i].Namespace == namespace && items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}

func findCronJob(items []batchv1.CronJob, namespace, name string) *batchv1.CronJob {
	for i := range items {
		if items[i].Namespace == namespace && items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}

func findService(items []corev1.Service, namespace, name string) *corev1.Service {
	for i := range items {
		if items[i].Namespace == namespace && items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}

func findIngress(items []networkingv1.Ingress, namespace, name string) *networkingv1.Ingress {
	for i := range items {
		if items[i].Namespace == namespace && items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}

func findPVC(items []corev1.PersistentVolumeClaim, namespace, name string) *corev1.PersistentVolumeClaim {
	for i := range items {
		if items[i].Namespace == namespace && items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}
