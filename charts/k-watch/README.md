# K-Watch

Read-only Kubernetes dashboard that maps how resources in a namespace connect and shows Pod changes live.

Pick a namespace and K-Watch draws its Deployments, StatefulSets, DaemonSets, Jobs, CronJobs, Services, Ingresses, PVCs, and PVs as a connected map. Click a resource to highlight the Ingress → Service → workload path and see its Pods and the nodes they run on. Pod creation, status changes, and deletion appear on the map as they happen.

- **Namespace map** with Pod counts and status per workload
- **Focus panel** with routes, Pods, nodes, and bound volumes
- **Live Pod changes** streamed over Server-Sent Events, including waiting reasons such as `CrashLoopBackOff`
- **Overview** of resource counts, node health, Pod status, and recent events, plus CPU and memory usage when metrics-server is installed
- **Cluster bookmarks** to move between K-Watch installations in different clusters

Source and full documentation: [github.com/klolarion/k_watch](https://github.com/klolarion/k_watch)

## Prerequisites

- Kubernetes 1.21 or later
- Helm 3.8 or later
- Permission to create a ServiceAccount, ClusterRole, and ClusterRoleBinding
- metrics-server, if you want node CPU and memory usage

## Install

```bash
helm install k-watch oci://ghcr.io/klolarion/charts/k-watch \
  --version 0.1.1 \
  --namespace k-watch \
  --create-namespace
```

Open the dashboard through a port-forward, then visit [http://localhost:8282](http://localhost:8282):

```bash
kubectl -n k-watch port-forward svc/k-watch 8282:8282
```

The image `ghcr.io/klolarion/k-watch` is published for `linux/amd64` and `linux/arm64`.

## Configuration

| Key | Default | Description |
|---|---|---|
| `image.repository` | `ghcr.io/klolarion/k-watch` | Container image |
| `image.tag` | `""` | Image tag. Defaults to the chart `appVersion` |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy |
| `imagePullSecrets` | `[]` | Secrets for pulling from a private registry |
| `nameOverride` | `""` | Overrides the chart name in resource names |
| `fullnameOverride` | `""` | Overrides the full resource name |
| `serviceAccount.annotations` | `{}` | Annotations on the ServiceAccount |
| `podAnnotations` | `{}` | Annotations on the Pod |
| `podSecurityContext` | UID/GID `65532`, non-root, `RuntimeDefault` seccomp | Pod security context |
| `securityContext` | read-only root filesystem, no privilege escalation, all capabilities dropped | Container security context |
| `service.type` | `ClusterIP` | Service type |
| `service.port` | `8282` | Service port |
| `service.annotations` | `{}` | Annotations on the Service |
| `ingress.enabled` | `false` | Creates an Ingress |
| `ingress.className` | `""` | IngressClass name |
| `ingress.annotations` | `{}` | Annotations on the Ingress |
| `ingress.hosts` | `k-watch.local`, path `/` | Ingress hosts and paths |
| `ingress.tls` | `[]` | Ingress TLS settings |
| `resources` | requests `100m` / `128Mi`, limits `500m` / `512Mi` | Container resources |
| `env` | `[]` | Extra environment variables in Kubernetes `EnvVar` form |
| `probes.liveness.enabled` | `true` | Liveness probe on `/api/healthz` |
| `probes.readiness.enabled` | `true` | Readiness probe on `/api/healthz` |
| `nodeSelector` | `{}` | Node selector |
| `tolerations` | `[]` | Tolerations |
| `affinity` | `{}` | Affinity rules |

### Ingress

```yaml
ingress:
  enabled: true
  className: traefik # your cluster's IngressClass
  hosts:
    - host: k-watch.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: k-watch-tls
      hosts:
        - k-watch.example.com
```

Live Pod updates use Server-Sent Events, so turn off response buffering and allow long-lived connections on the proxy in front of K-Watch.

## Permissions and access control

The chart creates a ServiceAccount bound to a read-only ClusterRole: `get`, `list`, and `watch` on Pods, Services, Namespaces, Nodes, PVs, PVCs, Events, Deployments, ReplicaSets, StatefulSets, DaemonSets, Jobs, CronJobs, Ingresses, and EndpointSlices, plus `get` and `list` on node and Pod metrics. Secrets and ConfigMaps are outside its permissions, as are writes, Pod exec, and Pod logs.

K-Watch has no built-in login. Anyone who can reach it sees what its ServiceAccount can read. Use `kubectl port-forward` for personal access, and put an authentication proxy, Ingress authentication, or a VPN in front of any shared address.

## Uninstall

```bash
helm uninstall k-watch --namespace k-watch
```

This removes the ClusterRole and ClusterRoleBinding as well. The `k-watch` namespace remains.

## License

[Apache License 2.0](https://github.com/klolarion/k_watch/blob/main/LICENSE)
