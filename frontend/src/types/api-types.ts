export const API_ENDPOINTS = {
  HEALTH: '/api/healthz',
  NAMESPACES: '/api/namespaces',
  OVERVIEW: '/api/cluster/overview'
} as const

export interface HealthResponse {
  status: string
  timestamp: string
}

export interface PodCounts {
  total: number
  running: number
  pending: number
  failed: number
  succeeded: number
  unknown: number
}

export interface NamespaceSummary {
  name: string
  pods: number
  unhealthyPods: number
  deployments: number
  statefulSets: number
  daemonSets: number
  jobs: number
  cronJobs: number
  services: number
  ingresses: number
  pvcs: number
}

export interface NamespaceListResponse {
  serverTime: string
  namespaces: NamespaceSummary[]
}

export interface MapNode {
  id: string
  type: string
  label: string
  namespace?: string
  replicas?: number
  available?: number
  pods?: PodCounts
  status?: string
}

export interface MapEdge {
  from: string
  to: string
  kind: string
}

export interface NamespaceMapResponse {
  serverTime: string
  namespace: string
  nodes: MapNode[]
  edges: MapEdge[]
  hiddenJobs: number
  finishedJobLimit: number
}

export interface PodRow {
  name: string
  phase: string
  node: string
  restarts: number
}

export interface ServiceRef {
  name: string
  type: string
  ports: string[]
}

export interface IngressRef {
  name: string
  hosts: string[]
}

export interface NodePlacement {
  name: string
  pods: number
  running: number
}

export interface VolumeRef {
  name: string
  status?: string
  size?: string
  workloads: string[]
}

export interface PodFlowEvent {
  type: 'ADDED' | 'MODIFIED' | 'DELETED'
  time: string
  pod: string
  phase: string
  reason?: string
  node?: string
  restarts: number
  deleting: boolean
  workloadId: string
  workloadKind: string
  workloadName: string
}

export interface FocusResponse {
  serverTime: string
  target: MapNode
  pods: PodRow[]
  podsTruncated: boolean
  podTotal: number
  podLimit: number
  services: ServiceRef[]
  ingresses: IngressRef[]
  nodes: NodePlacement[]
  routes: MapEdge[]
  highlight: string[]
  volume?: VolumeRef
}

export interface OverviewResponse {
  info: {
    node: number
    namespace: number
    pod: number
    deployment: number
    service: number
    ingress: number
    replicaSet: number
    statefulSet: number
    daemonSet: number
    job: number
    cronJob: number
    pv: number
    pvc: number
  }
  podStatus?: { name: string; value: number }[]
  workloadDistribution?: { name: string; pods: number; services: number }[]
  nodeStatus?: { node: string; status: string; pods: number; cpu?: number; memory?: number }[]
  eventTimeline?: { time: string; event: string; type: string; namespace: string; object: string; message: string }[]
}

export function focusKind(type: string): string | null {
  switch (type) {
    case 'Deployment':
      return 'deployment'
    case 'StatefulSet':
      return 'statefulset'
    case 'DaemonSet':
      return 'daemonset'
    case 'Job':
      return 'job'
    case 'CronJob':
      return 'cronjob'
    case 'Service':
      return 'service'
    case 'Ingress':
      return 'ingress'
    case 'PersistentVolumeClaim':
      return 'pvc'
    case 'PodGroup':
      return 'podgroup'
    default:
      return null
  }
}
