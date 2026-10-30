import { useEffect, useMemo, useRef, useState } from 'react'
import cytoscape from 'cytoscape'
import CytoscapeComponent from 'react-cytoscapejs'
import dagre from 'cytoscape-dagre'
import { getNamespaceMap, getNamespaces, getWorkloadFocus, handleApiError } from '../services/api'
import { focusKind } from '../types/api-types'
import type { FocusResponse, MapEdge, MapNode, NamespaceMapResponse, NamespaceSummary, PodFlowEvent } from '../types/api-types'

cytoscape.use(dagre)

const layout = {
  name: 'dagre',
  rankDir: 'TB',
  nodeSep: 28,
  rankSep: 64,
  fit: true,
  padding: 24,
  animate: false,
  nodeDimensionsIncludeLabels: true
} as cytoscape.LayoutOptions

const typeColor: Record<string, string> = {
  Namespace: '#3498DB',
  Deployment: '#F39C12',
  StatefulSet: '#E67E92',
  DaemonSet: '#E74C3C',
  Job: '#1ABC9C',
  CronJob: '#16A085',
  Service: '#9B59B6',
  Ingress: '#2980B9',
  PersistentVolume: '#34495E',
  PersistentVolumeClaim: '#5D6D7E',
  PodGroup: '#7F8C8D'
}

const legend = [
  ['Namespace', 'Namespace'],
  ['Deployment', 'Deployment'],
  ['StatefulSet', 'StatefulSet'],
  ['DaemonSet', 'DaemonSet'],
  ['Job', 'Job'],
  ['CronJob', 'CronJob'],
  ['Service', 'Service'],
  ['Ingress', 'Ingress'],
  ['PVC', 'PersistentVolumeClaim'],
  ['PV', 'PersistentVolume'],
  ['Unmanaged', 'PodGroup']
] as const

function displayLabel(node: MapNode): string {
  const lines = [node.label]
  if (node.pods && node.pods.total > 0) {
    lines.push(`${node.pods.running}/${node.pods.total}`)
  }
  if (node.status) {
    lines.push(node.status)
  }
  return lines.join('\n')
}

function toElements(map: NamespaceMapResponse): cytoscape.ElementDefinition[] {
  return [
    ...map.nodes.map((node) => ({
      data: {
        id: node.id,
        label: displayLabel(node),
        rawLabel: node.label,
        type: node.type
      }
    })),
    ...map.edges.map((edge) => ({
      data: {
        id: `${edge.kind}:${edge.from}>${edge.to}`,
        source: edge.from,
        target: edge.to,
        kind: edge.kind
      }
    }))
  ]
}

function objectLabel(id: string): string {
  const name = id.includes('/') ? id.slice(id.lastIndexOf('/') + 1) : id.slice(id.indexOf(':') + 1)
  const prefix = id.slice(0, id.indexOf(':'))
  const kinds: Record<string, string> = {
    ns: 'Namespace',
    deploy: 'Deployment',
    sts: 'StatefulSet',
    ds: 'DaemonSet',
    job: 'Job',
    cronjob: 'CronJob',
    svc: 'Service',
    ing: 'Ingress',
    pvc: 'PVC',
    pv: 'PV',
    podgroup: 'Unmanaged'
  }
  return `${kinds[prefix] ?? prefix} ${name}`
}

function flowVerb(event: PodFlowEvent) {
  if (event.type === 'DELETED') return 'Deleted'
  if (event.deleting) return 'Deleting'
  if (event.type === 'ADDED') return 'Created'
  return 'Status'
}

function flowTone(event: PodFlowEvent) {
  if (event.type === 'DELETED' || event.deleting) return 'deleted'
  if (event.type === 'ADDED') return 'added'
  return 'modified'
}

function shortPodName(name: string) {
  return name.length > 18 ? `${name.slice(0, 16)}…` : name
}

function flowShift(pod: string) {
  let hash = 0
  for (let i = 0; i < pod.length; i++) hash = (hash * 33 + pod.charCodeAt(i)) >>> 0
  return { x: (hash % 5 - 2) * 36, y: 72 + (Math.floor(hash / 5) % 2) * 28 }
}

function showLivePod(cy: cytoscape.Core, event: PodFlowEvent, namespace: string, timers: Map<string, number>) {
  if (cy.destroyed()) return
  const parentId = cy.getElementById(event.workloadId).nonempty() ? event.workloadId : `ns:${namespace}`
  const parent = cy.getElementById(parentId)
  if (parent.empty()) return
  const id = `live:${event.pod}`
  const tone = flowTone(event)
  // A wait reason such as CrashLoopBackOff says more than the phase, so it wins when present.
  const phase = event.deleting && event.type !== 'DELETED' ? 'Deleting' : (event.reason || event.phase || flowVerb(event))
  const label = `${shortPodName(event.pod)}\n${flowVerb(event)}\n${phase}`
  const pos = parent.position()
  const shift = flowShift(event.pod)

  cy.batch(() => {
    const node = cy.getElementById(id)
    if (node.empty()) {
      cy.add({
        data: { id, label, type: 'FlowPod', rawLabel: event.pod },
        position: { x: pos.x + shift.x, y: pos.y + shift.y },
        classes: `flow-${tone}`
      })
      cy.add({
        data: { id: `flow:${parentId}>${event.pod}`, source: parentId, target: id, kind: 'flow' },
        classes: `flow-${tone}`
      })
    } else {
      node.data('label', label)
      node.removeClass('flow-added flow-modified flow-deleted')
      node.addClass(`flow-${tone}`)
    }
    parent.removeClass('flow-added flow-modified flow-deleted')
    parent.addClass(`flow-${tone}`)
    const live = cy.nodes('[type = "FlowPod"]')
    if (live.length > 12) live[0].remove()
  })

  window.setTimeout(() => {
    if (cy.destroyed()) return
    const current = cy.getElementById(parentId)
    if (current.nonempty()) current.removeClass(`flow-${tone}`)
  }, 1600)

  const previous = timers.get(event.pod)
  if (previous) window.clearTimeout(previous)
  timers.set(event.pod, window.setTimeout(() => {
    timers.delete(event.pod)
    if (cy.destroyed()) return
    const node = cy.getElementById(id)
    if (node.nonempty()) node.remove()
  }, event.type === 'DELETED' ? 2200 : 7000))
}

function applyFocus(cy: cytoscape.Core, focus: FocusResponse | null) {
  cy.batch(() => {
    cy.edges('[kind = "routes"]').remove()
    cy.elements().removeClass('faded').removeClass('focused')
    if (!focus) return
    const highlight = new Set(focus.highlight)
    focus.routes.forEach((edge: MapEdge) => {
      if (cy.getElementById(edge.from).empty() || cy.getElementById(edge.to).empty()) return
      const id = `routes:${edge.from}>${edge.to}`
      if (cy.getElementById(id).empty()) {
        cy.add({ data: { id, source: edge.from, target: edge.to, kind: 'routes' } })
      }
    })
    cy.nodes().forEach((node) => {
      if (node.data('type') === 'FlowPod') return
      if (!highlight.has(node.id())) node.addClass('faded')
      if (node.id() === focus.target.id) node.addClass('focused')
    })
    cy.edges().forEach((edge) => {
      if (edge.data('kind') === 'flow') return
      if (!highlight.has(edge.source().id()) || !highlight.has(edge.target().id())) {
        edge.addClass('faded')
      }
    })
  })
}

interface NamespaceMapProps {
  isConnected: boolean
  refreshKey: number
}

function NamespaceMapView({ isConnected, refreshKey }: NamespaceMapProps) {
  const cyRef = useRef<cytoscape.Core | null>(null)
  const requestRef = useRef(0)
  const [namespaces, setNamespaces] = useState<NamespaceSummary[]>([])
  const [query, setQuery] = useState('')
  const [listError, setListError] = useState('')
  const [loadingList, setLoadingList] = useState(false)
  const [selected, setSelected] = useState<string | null>(null)
  const [map, setMap] = useState<NamespaceMapResponse | null>(null)
  const [mapError, setMapError] = useState('')
  const [loadingMap, setLoadingMap] = useState(false)
  const [focus, setFocus] = useState<FocusResponse | null>(null)
  const [focusError, setFocusError] = useState('')
  const [loadingFocus, setLoadingFocus] = useState(false)
  const [flows, setFlows] = useState<PodFlowEvent[]>([])
  const [live, setLive] = useState(false)
  const [flowError, setFlowError] = useState('')
  const flowTimers = useRef<Map<string, number>>(new Map())

  const elements = useMemo(() => (map ? toElements(map) : []), [map])

  useEffect(() => {
    if (!isConnected) return
    let cancelled = false
    setLoadingList(true)
    setListError('')
    getNamespaces()
      .then((data) => {
        if (!cancelled) setNamespaces(data.namespaces ?? [])
      })
      .catch((error) => {
        if (!cancelled) setListError(handleApiError(error))
      })
      .finally(() => {
        if (!cancelled) setLoadingList(false)
      })
    return () => {
      cancelled = true
    }
  }, [isConnected, refreshKey])

  useEffect(() => {
    if (!isConnected || !selected) return
    let cancelled = false
    requestRef.current += 1
    setFocus(null)
    setFocusError('')
    setLoadingMap(true)
    setMapError('')
    getNamespaceMap(selected)
      .then((data) => {
        if (!cancelled) setMap(data)
      })
      .catch((error) => {
        if (!cancelled) {
          setMap(null)
          setMapError(handleApiError(error))
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingMap(false)
      })
    return () => {
      cancelled = true
    }
  }, [isConnected, refreshKey, selected])

  useEffect(() => {
    const cy = cyRef.current
    if (!cy || cy.destroyed()) return
    applyFocus(cy, focus)
  }, [focus, map])

  useEffect(() => {
    if (!isConnected || !selected) return
    const namespace = selected
    const timers = flowTimers.current
    setFlows([])
    setLive(false)
    setFlowError('')
    const source = new EventSource(`/api/namespaces/${encodeURIComponent(namespace)}/flow`)
    source.onopen = () => setLive(true)
    source.onerror = () => setLive(false)
    // The server sends "problem" when the pod watch cannot continue, such as a missing permission.
    // Retrying would hit the same error, so stop here and let the refresh button reconnect.
    source.addEventListener('problem', (event) => {
      let message = 'Live updates stopped.'
      try {
        const body = JSON.parse((event as MessageEvent).data) as { error?: string }
        if (body.error) message = `Live updates stopped: ${body.error}`
      } catch {
        // keep the generic message
      }
      source.close()
      setLive(false)
      setFlowError(message)
    })
    source.addEventListener('flow', (event) => {
      const message = event as MessageEvent
      const flow = JSON.parse(message.data) as PodFlowEvent
      setFlows((current) => [flow, ...current].slice(0, 8))
      const cy = cyRef.current
      if (!cy || cy.destroyed()) return
      showLivePod(cy, flow, namespace, timers)
    })
    return () => {
      source.close()
      setLive(false)
      timers.forEach((timer) => window.clearTimeout(timer))
      timers.clear()
      const cy = cyRef.current
      if (cy && !cy.destroyed()) cy.nodes('[type = "FlowPod"]').remove()
    }
  }, [isConnected, refreshKey, selected])

  const openFocus = (namespace: string, kind: string, name: string) => {
    const request = ++requestRef.current
    setLoadingFocus(true)
    setFocusError('')
    getWorkloadFocus(namespace, kind, name)
      .then((data) => {
        if (request === requestRef.current) setFocus(data)
      })
      .catch((error) => {
        if (request !== requestRef.current) return
        setFocus(null)
        setFocusError(handleApiError(error))
      })
      .finally(() => {
        if (request === requestRef.current) setLoadingFocus(false)
      })
  }

  const closeFocus = () => {
    requestRef.current += 1
    setFocus(null)
    setFocusError('')
    setLoadingFocus(false)
  }

  const onCy = (cy: cytoscape.Core) => {
    if (cyRef.current === cy) return
    cyRef.current = cy
    cy.on('tap', 'node', (event) => {
      const type = event.target.data('type') as string
      const kind = focusKind(type)
      const namespace = selected
      if (!kind || !namespace) return
      const name = type === 'PodGroup' ? 'unmanaged' : (event.target.data('rawLabel') as string)
      openFocus(namespace, kind, name)
    })
    cy.on('tap', (event) => {
      if (event.target === cy) closeFocus()
    })
  }

  if (!isConnected) {
    return (
      <div className="no-data">
        <p>Cannot reach the K-Watch server. The map opens once it responds.</p>
      </div>
    )
  }

  if (!selected) {
    const shown = namespaces.filter((ns) => ns.name.includes(query.trim()))
    return (
      <div className="namespace-page">
        <div className="namespace-toolbar">
          <div>
            <h2>Namespaces</h2>
            <p>Pick a namespace to see how its resources connect.</p>
          </div>
          <input
            className="namespace-search"
            value={query}
            placeholder="Filter by name"
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>
        {listError && <p className="map-error">{listError}</p>}
        {loadingList && namespaces.length === 0 ? (
          <div className="loading-container"><p>Loading namespaces...</p></div>
        ) : (
          <div className="namespace-table-wrap">
            <table className="namespace-table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Pods</th>
                  <th>Unhealthy</th>
                  <th>Deploy</th>
                  <th>STS</th>
                  <th>DS</th>
                  <th>Job</th>
                  <th>Cron</th>
                  <th>Svc</th>
                  <th>Ing</th>
                  <th>PVC</th>
                </tr>
              </thead>
              <tbody>
                {shown.map((ns) => (
                  <tr key={ns.name} onClick={() => setSelected(ns.name)}>
                    <td>
                      <button type="button" className="namespace-link" onClick={() => setSelected(ns.name)}>
                        {ns.name}
                      </button>
                    </td>
                    <td>{ns.pods}</td>
                    <td className={ns.unhealthyPods > 0 ? 'unhealthy' : ''}>{ns.unhealthyPods}</td>
                    <td>{ns.deployments}</td>
                    <td>{ns.statefulSets}</td>
                    <td>{ns.daemonSets}</td>
                    <td>{ns.jobs}</td>
                    <td>{ns.cronJobs}</td>
                    <td>{ns.services}</td>
                    <td>{ns.ingresses}</td>
                    <td>{ns.pvcs}</td>
                  </tr>
                ))}
                {shown.length === 0 && (
                  <tr>
                    <td colSpan={11}>No namespaces match.</td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        )}
      </div>
    )
  }

  return (
    <div className="map-page">
      <div className="namespace-toolbar">
        <button type="button" className="text-button" onClick={() => { setSelected(null); setMap(null); closeFocus() }}>
          Namespaces
        </button>
        <h2>{selected}</h2>
        {map && map.hiddenJobs > 0 && (
          <p className="map-note">
            {map.hiddenJobs} finished jobs are hidden. Active jobs stay, and only the {map.finishedJobLimit} newest finished jobs without a CronJob stay on the map.
          </p>
        )}
        {map && map.nodes.length > 300 && (
          <p className="map-note">This namespace has {map.nodes.length} objects. Click a resource to focus on its connections.</p>
        )}
      </div>
      {mapError && <p className="map-error">{mapError}</p>}
      <div className="flow-strip" aria-live="polite">
        <span className={live ? 'live-dot on' : 'live-dot'}>{live ? 'Live' : flowError ? 'Stopped' : 'Connecting'}</span>
        {flowError && <span className="map-error">{flowError}</span>}
        {flowError ? null : flows.length === 0 ? (
          <span className="flow-empty">Waiting for pod creates, deletes, and status changes.</span>
        ) : flows.map((flow) => (
          <span key={`${flow.time}-${flow.pod}-${flow.type}-${flow.phase}`} className={`flow-chip ${flowTone(flow)}`}>
            <strong>{flowVerb(flow)}</strong>
            {shortPodName(flow.pod)}
            <em>{flow.deleting && flow.type !== 'DELETED' ? 'Deleting' : (flow.reason || flow.phase)}</em>
            <span>{flow.workloadName}</span>
          </span>
        ))}
      </div>
      {loadingMap && !map ? (
        <div className="loading-container"><p>Loading namespace map...</p></div>
      ) : map ? (
        <div className="cluster-graph-container">
          <div className="left-panel">
            <div className="graph-legend">
              <h3>Resource Types</h3>
              <div className="legend-items">
                {legend.map(([label, type]) => (
                  <div key={type} className="legend-item">
                    <div className="legend-color" style={{ backgroundColor: typeColor[type] }}></div>
                    <span className="legend-label">{label}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
          <div className="graph-container">
            <CytoscapeComponent
              key={selected}
              elements={elements}
              stylesheet={stylesheet}
              layout={layout}
              cy={onCy}
              style={{ width: '100%', height: '100%' }}
            />
          </div>
          <aside className="focus-panel">
            <FocusPanel
              focus={focus}
              loading={loadingFocus}
              error={focusError}
              onClose={closeFocus}
            />
          </aside>
        </div>
      ) : null}
    </div>
  )
}

function FocusPanel({
  focus,
  loading,
  error,
  onClose
}: {
  focus: FocusResponse | null
  loading: boolean
  error: string
  onClose: () => void
}) {
  if (loading && !focus) {
    return <p>Loading selection...</p>
  }
  if (error) {
    return <p className="map-error">{error}</p>
  }
  if (!focus) {
    return (
      <div>
        <h3>Selection</h3>
        <p>Select a workload, service, ingress, or volume. Routes are drawn on the map. The nodes running those pods stay in this list.</p>
      </div>
    )
  }
  const counts = focus.target.pods
  return (
    <div>
      <div className="focus-header">
        <div>
          <p className="focus-kind">{focus.target.type}</p>
          <h3>{focus.target.label}</h3>
        </div>
        <button type="button" className="text-button" onClick={onClose}>Close</button>
      </div>
      {counts && (
        <p className="focus-counts">
          {counts.running} running · {counts.pending} pending · {counts.failed} failed · {counts.succeeded} succeeded
        </p>
      )}
      {focus.volume && (
        <section>
          <h4>Volume</h4>
          <p>{focus.volume.name || 'Unbound'} {focus.volume.status} {focus.volume.size}</p>
          {focus.volume.workloads.length > 0 && <p>{focus.volume.workloads.join(', ')}</p>}
        </section>
      )}
      <section>
        <h4>Routes</h4>
        {focus.routes.length === 0 ? <p>No service routes for this selection.</p> : (
          <ul>
            {focus.routes.map((edge) => (
              <li key={`${edge.from}>${edge.to}`}>{objectLabel(edge.from)} → {objectLabel(edge.to)}</li>
            ))}
          </ul>
        )}
      </section>
      <section>
        <h4>Nodes</h4>
        {focus.nodes.length === 0 ? <p>No scheduled pods.</p> : (
          <ul>
            {focus.nodes.map((node) => (
              <li key={node.name}>{node.name} · {node.running}/{node.pods} running</li>
            ))}
          </ul>
        )}
      </section>
      <section>
        <h4>Pods</h4>
        {focus.podsTruncated && <p>Showing {focus.pods.length} of {focus.podTotal}.</p>}
        {focus.pods.length === 0 ? <p>No pods.</p> : (
          <ul className="pod-list">
            {focus.pods.map((pod) => (
              <li key={pod.name}>
                <span className={`phase-badge phase-${pod.phase.toLowerCase()}`}>{pod.phase}</span>
                <span>{pod.name}</span>
                <span className="pod-meta">{pod.node}{pod.restarts > 0 ? ` · ${pod.restarts} restarts` : ''}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}

const stylesheet = [
  {
    selector: 'node',
    style: {
      label: 'data(label)',
      'text-wrap': 'wrap',
      'text-max-width': 110,
      'font-size': 11,
      'text-valign': 'bottom',
      'text-halign': 'center',
      'text-margin-y': 6,
      color: '#1f2937',
      'background-color': '#95A5A6',
      width: 26,
      height: 26
    }
  },
  ...Object.entries(typeColor).map(([type, color]) => ({
    selector: `node[type = "${type}"]`,
    style: { 'background-color': color }
  })),
  {
    selector: 'edge',
    style: {
      width: 1.5,
      'line-color': '#95A5A6',
      'target-arrow-color': '#95A5A6',
      'target-arrow-shape': 'triangle',
      'curve-style': 'bezier',
      'arrow-scale': 0.7
    }
  },
  {
    selector: 'edge[kind = "contains"]',
    style: { 'line-color': '#3498DB', 'target-arrow-color': '#3498DB' }
  },
  {
    selector: 'edge[kind = "owns"]',
    style: { 'line-color': '#F39C12', 'target-arrow-color': '#F39C12', 'line-style': 'dashed' }
  },
  {
    selector: 'edge[kind = "binds"]',
    style: { 'line-color': '#34495E', 'target-arrow-color': '#34495E' }
  },
  {
    selector: 'edge[kind = "routes"]',
    style: { width: 2.5, 'line-color': '#8E44AD', 'target-arrow-color': '#8E44AD' }
  },
  {
    selector: '.faded',
    style: { opacity: 0.15 }
  },
  {
    selector: 'node.focused',
    style: { 'border-width': 3, 'border-color': '#111827', 'border-opacity': 1 }
  },
  {
    selector: 'node[type = "FlowPod"]',
    style: {
      width: 16,
      height: 16,
      'font-size': 9,
      'text-max-width': 88,
      'background-color': '#64748b'
    }
  },
  {
    selector: 'edge[kind = "flow"]',
    style: { width: 1.5, 'line-style': 'dashed', 'line-color': '#64748b', 'target-arrow-color': '#64748b' }
  },
  {
    selector: 'node.flow-added',
    style: { 'border-width': 3, 'border-color': '#10b981', 'border-opacity': 1 }
  },
  {
    selector: 'node[type = "FlowPod"].flow-added',
    style: { 'background-color': '#10b981' }
  },
  {
    selector: 'edge.flow-added',
    style: { 'line-color': '#10b981', 'target-arrow-color': '#10b981' }
  },
  {
    selector: 'node.flow-modified',
    style: { 'border-width': 3, 'border-color': '#d97706', 'border-opacity': 1 }
  },
  {
    selector: 'node[type = "FlowPod"].flow-modified',
    style: { 'background-color': '#d97706' }
  },
  {
    selector: 'edge.flow-modified',
    style: { 'line-color': '#d97706', 'target-arrow-color': '#d97706' }
  },
  {
    selector: 'node.flow-deleted',
    style: { 'border-width': 3, 'border-color': '#dc2626', 'border-opacity': 1 }
  },
  {
    selector: 'node[type = "FlowPod"].flow-deleted',
    style: { 'background-color': '#dc2626' }
  },
  {
    selector: 'edge.flow-deleted',
    style: { 'line-color': '#dc2626', 'target-arrow-color': '#dc2626' }
  },
] as cytoscape.StylesheetJson

export default NamespaceMapView
