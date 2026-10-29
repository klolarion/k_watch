import {
  Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  Bar, PieChart, Pie, Cell,
  ComposedChart
} from 'recharts'
import type { OverviewResponse } from '../types/api-types'

interface OverviewProps {
  isConnected: boolean
  overviewData: OverviewResponse | null
  overviewError: string
  lastCheck: Date | null
}

const podStatusColor = (status: string) => {
  const colorMap: Record<string, string> = {
    Running: '#10b981',
    Pending: '#f59e0b',
    Failed: '#ef4444',
    Succeeded: '#3b82f6',
    Unknown: '#6b7280'
  }
  return colorMap[status] || '#6b7280'
}

// Usage is missing when the cluster has no metrics-server. Show that instead of a fake 0%.
function UsageBar({ label, percent }: { label: string; percent?: number }) {
  if (percent === undefined) {
    return (
      <div className="overview-heatmap-bar">
        <span>{label}: no metrics</span>
      </div>
    )
  }
  return (
    <div className="overview-heatmap-bar">
      <span>{label}: {percent}%</span>
      <div
        className={`overview-heatmap-fill ${percent < 30 ? 'low' : percent < 70 ? 'medium' : 'high'}`}
        style={{ width: `${Math.min(percent, 100)}%` }}
      />
    </div>
  )
}

// Event times arrive in UTC. Show them in the viewer's time zone, with the date when it is not today.
function formatEventTime(value: string) {
  const time = new Date(value)
  if (Number.isNaN(time.getTime())) return value
  const clock = time.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  if (time.toDateString() === new Date().toDateString()) return clock
  return `${time.toLocaleDateString([], { month: '2-digit', day: '2-digit' })} ${clock}`
}

function Overview({ isConnected, overviewData, overviewError, lastCheck }: OverviewProps) {
  if (!isConnected) {
    return (
      <div className="overview-container">
        <div className="overview-header">
          <h2>Cluster Overview</h2>
          <div className="connection-status-disconnected">Cannot reach the K-Watch server</div>
        </div>
        <div className="connection-message-large">
          The overview loads once the server responds. Health is checked every 30 seconds.
        </div>
      </div>
    )
  }

  if (!overviewData && overviewError) {
    return (
      <div className="overview-container">
        <p className="map-error">Could not load the overview: {overviewError}</p>
      </div>
    )
  }

  if (!overviewData) {
    return (
      <div className="overview-container">
        <div className="loading-container"><p>Loading overview...</p></div>
      </div>
    )
  }

  const podStatus = overviewData.podStatus ?? []
  const workload = overviewData.workloadDistribution ?? []
  const nodes = overviewData.nodeStatus ?? []
  const events = overviewData.eventTimeline ?? []

  return (
    <div className="overview-container">
      <div className="overview-summary-cards">
        <div className="overview-summary-card">
          <div className="overview-header-row">
            <h3>Cluster Info</h3>
            <div className="cluster-status-info">
              <div className="status-indicator connected">Connected</div>
              {lastCheck && (
                <span className="last-check-time">
                  Last check: {lastCheck.toLocaleTimeString('en-US', { hour12: false })}
                </span>
              )}
            </div>
          </div>
          <div className="overview-resource-counts">
            {([
              ['Node', overviewData.info.node],
              ['Namespace', overviewData.info.namespace],
              ['Pod', overviewData.info.pod],
              ['Deployment', overviewData.info.deployment],
              ['Service', overviewData.info.service],
              ['Ingress', overviewData.info.ingress],
              ['ReplicaSet', overviewData.info.replicaSet],
              ['StatefulSet', overviewData.info.statefulSet],
              ['DaemonSet', overviewData.info.daemonSet],
              ['Job', overviewData.info.job],
              ['CronJob', overviewData.info.cronJob],
              ['PV', overviewData.info.pv],
              ['PVC', overviewData.info.pvc]
            ] as const).map(([label, count]) => (
              <div key={label} className="overview-count-item">
                <span className="overview-count">{count || 0}</span>
                <span className="overview-label">{label}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="overview-grid">
        <div className="overview-chart-card">
          <h3>Pod Status Distribution</h3>
          {podStatus.length === 0 ? <p>No pod status data</p> : (
            <ResponsiveContainer width="100%" height={200}>
              <PieChart>
                <Pie data={podStatus} cx="50%" cy="50%" outerRadius={60} dataKey="value" label={({ name, value }: { name?: string; value?: number }) => `${name}: ${value}`}>
                  {podStatus.map((entry) => (
                    <Cell key={entry.name} fill={podStatusColor(entry.name)} />
                  ))}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          )}
        </div>

        <div className="overview-chart-card">
          <h3>Workload by Namespace</h3>
          {workload.length === 0 ? <p>No workload data</p> : (
            <ResponsiveContainer width="100%" height={200}>
              <ComposedChart data={workload}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="name" />
                <YAxis />
                <Tooltip />
                <Bar dataKey="pods" fill="#3b82f6" />
                <Line type="monotone" dataKey="services" stroke="#10b981" />
              </ComposedChart>
            </ResponsiveContainer>
          )}
        </div>

        <div className="overview-chart-card">
          <h3>Node Status</h3>
          <div className="overview-node-status-container">
            {nodes.length === 0 ? <p>No node status data</p> : nodes.map((node) => (
              <div key={node.node} className="overview-node-status-item">
                <div className="overview-node-status-header">
                  <span className="overview-node-name">{node.node}</span>
                  <div className="overview-node-info">
                    <span className={`overview-node-status-badge ${node.status.toLowerCase()}`}>{node.status}</span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className="overview-chart-card">
          <h3>Node Workload</h3>
          <div className="overview-heatmap-container">
            {nodes.length === 0 ? <p>No node workload data</p> : nodes.map((node) => (
              <div key={node.node} className="overview-heatmap-row">
                <div className="overview-heatmap-label">{node.node}</div>
                <div className="overview-heatmap-bars">
                  <div className="overview-heatmap-bar">
                    <span>Pods: {node.pods}</span>
                    <div
                      className={`overview-heatmap-fill ${node.pods < 20 ? 'low' : node.pods < 50 ? 'medium' : 'high'}`}
                      style={{ width: `${Math.min((node.pods / 50) * 100, 100)}%` }}
                    />
                  </div>
                  <UsageBar label="CPU" percent={node.cpu} />
                  <UsageBar label="Memory" percent={node.memory} />
                </div>
              </div>
            ))}
          </div>
          {nodes.length > 0 && nodes.every((node) => node.cpu === undefined && node.memory === undefined) && (
            <p className="overview-note">CPU and memory usage need metrics-server in this cluster.</p>
          )}
        </div>

        <div className="overview-chart-card overview-chart-wide">
          <h3>Event Timeline</h3>
          <div className="overview-timeline-container">
            {events.length === 0 ? <p>No events</p> : events.map((event, index) => (
              <div key={`${event.time}-${event.object}-${index}`} className="overview-timeline-item">
                <div className="overview-timeline-item-content">
                  <div className="overview-timeline-time" title={event.time}>{formatEventTime(event.time)}</div>
                  <div className="overview-timeline-event">{event.event}</div>
                  <div className="overview-timeline-namespace">{event.namespace}</div>
                  <div className="overview-timeline-object">{event.object}</div>
                  <div className={`overview-timeline-type ${(event.type || '').toLowerCase()}`}>{event.type}</div>
                  <div className="overview-timeline-message">{event.message}</div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

export default Overview
