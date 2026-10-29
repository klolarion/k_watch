<p align="center"><a href="README.md">English</a> | <a href="README.ko.md">한국어</a></p>

<p align="center">
  <img src="frontend/public/k-watch-logo.svg" alt="K-Watch 로고" width="112" />
</p>

<h1 align="center">K-Watch</h1>

<p align="center">
  <strong>네임스페이스의 연결 구조와 Pod의 변화를 한눈에.</strong><br />
  Kubernetes 리소스 관계를 지도로 탐색하는 읽기 전용 대시보드
</p>

<p align="center">
  <a href="https://github.com/klolarion/k_watch/actions/workflows/ci.yml"><img src="https://github.com/klolarion/k_watch/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue" alt="Apache License 2.0" /></a>
  <a href="charts/k-watch"><img src="https://img.shields.io/badge/install-Helm-0F1689" alt="Helm 설치" /></a>
</p>

---

“이 Service는 어떤 워크로드로 연결되지?”, “이 Pod는 어느 노드에서 실행 중이지?”, “배포 후 Pod 상태가 어떻게 바뀌고 있지?”

K-Watch는 **네임스페이스 하나에 집중해 리소스 사이의 관계를 보여줍니다.** Ingress부터 Service, 워크로드, 스토리지까지 연결을 따라가고, Pod의 생성·상태 변화·삭제를 같은 지도에서 확인할 수 있습니다. 클러스터마다 하나의 Pod로 설치하며 브라우저로 접속합니다.

[주요 기능](#주요-기능) · [빠른 시작](#빠른-시작) · [사용 방법](#사용-방법) · [설정](#설정) · [권한과 접근 제어](#권한과-접근-제어) · [문제 해결](#문제-해결) · [개발](#개발)

## 주요 기능

### 네임스페이스별 리소스 지도

Deployment, StatefulSet, DaemonSet, Job, CronJob과 Service, Ingress, PVC, PV의 관계를 그래프로 표시합니다. 개별 Pod를 모두 펼치는 대신 워크로드별 Pod 수와 상태를 요약해, 리소스가 많은 네임스페이스에서도 연결 구조에 집중할 수 있습니다.

- **연결 탐색** — 리소스를 클릭하면 관련 경로를 강조하고 상세 패널을 엽니다.
- **Pod 상세 정보** — 선택한 리소스에 연결된 Pod의 상태, 실행 노드, 재시작 횟수와 Pod가 올라간 노드 목록을 확인합니다.
- **스토리지 관계** — PVC가 어떤 PV에 바인딩됐는지 지도에 표시합니다. PVC를 선택하면 PV 상태와 용량을 보여주고 그 PVC를 사용하는 워크로드를 강조합니다.
- **완료된 Job 정리** — CronJob이 만든 완료 Job은 숨기고, 독립적으로 생성된 완료 Job은 최신 20개만 표시합니다. 실행 중인 Job은 유지합니다.

### 실시간 Pod 변화

선택한 네임스페이스의 Pod 이벤트를 Kubernetes Watch와 SSE(Server-Sent Events)로 전달합니다. Pod가 생성되거나 삭제되고 상태가 바뀌면 지도 위에 일시적으로 나타나며, `CrashLoopBackOff` 같은 컨테이너 대기 사유도 함께 표시합니다.

> 실시간 스트림은 Pod 변화를 보여줍니다. 워크로드·Service 등 리소스 구성과 요약 수치를 다시 조회하려면 상단 새로고침 버튼을 사용하세요. 표시되는 연결은 Kubernetes 리소스 설정을 바탕으로 하며 실제 네트워크 트래픽 측정값은 아닙니다.

### 클러스터 Overview와 네임스페이스 목록

- **Overview** — 클러스터 전체 리소스 수, 노드 상태, Pod 상태 분포, 네임스페이스별 워크로드, 최근 이벤트를 확인합니다.
- **노드 사용률** — metrics-server가 제공하는 CPU·메모리 사용률을 표시합니다. 메트릭 수집은 선택 사항입니다.
- **네임스페이스 목록** — 이름으로 검색하고, Pod 수·비정상 Pod 수와 리소스별 개수를 비교해 살펴볼 네임스페이스를 고릅니다.
- **클러스터 주소 북마크** — 다른 클러스터에 설치한 K-Watch 주소를 저장하고 해당 설치로 이동합니다.

## 빠른 시작

### 준비 사항

- Kubernetes 클러스터와 해당 클러스터에 연결된 `kubectl`
- OCI 차트를 설치할 수 있는 Helm 3.8 이상
- ServiceAccount, ClusterRole, ClusterRoleBinding을 포함한 차트 리소스를 생성할 권한
- 노드 CPU·메모리 사용률이 필요하다면 metrics-server

차트의 Kubernetes 버전 조건은 `>=1.21.0-0`입니다. 배포 이미지 빌드 대상은 `linux/amd64`, `linux/arm64`입니다.

### 1. Helm으로 설치

```bash
helm install k-watch oci://ghcr.io/klolarion/charts/k-watch \
  --version 0.1.0 \
  --namespace k-watch \
  --create-namespace
```

소스 저장소에서 차트를 직접 설치할 수도 있습니다.

```bash
helm install k-watch ./charts/k-watch --namespace k-watch --create-namespace
```

### 2. 로컬 포트로 연결

```bash
kubectl -n k-watch rollout status deployment/k-watch
kubectl -n k-watch port-forward svc/k-watch 8282:8282
```

### 3. 브라우저에서 열기

[http://localhost:8282](http://localhost:8282)에 접속합니다. `port-forward` 명령은 접속하는 동안 실행해 두세요.

> 위 명령은 Bash 기준입니다. PowerShell에서는 여러 줄 명령의 `\`를 제거하고 한 줄로 실행하세요.

## 사용 방법

1. **Overview에서 전체 상태를 확인합니다.** 노드 상태, Pod 분포, 최근 이벤트로 살펴볼 대상을 좁힙니다.
2. **Map에서 네임스페이스를 선택합니다.** 목록의 `Unhealthy` 열로 비정상 Pod가 있는 네임스페이스를 찾거나 이름으로 검색합니다.
3. **지도에서 리소스를 클릭합니다.** 연결된 경로와 우측 패널의 Pod·노드·볼륨 정보를 확인합니다. Pod 목록은 최대 50개를 표시하며 전체 개수도 함께 안내합니다.
4. **Live 표시와 Pod 이벤트를 확인합니다.** 생성·삭제·상태 변화를 관찰하고, 리소스 구성이 바뀌었다면 상단 새로고침 버튼으로 지도를 갱신합니다.

### 여러 클러스터 오가기

K-Watch는 **설치된 클러스터 하나**를 조회합니다. 여러 클러스터를 사용하려면 각 클러스터에 설치한 뒤, 상단 **Cluster → Add**에서 이름과 접속 주소를 저장하세요. 선택하면 해당 K-Watch 주소로 이동합니다.

로컬에서 여러 설치에 접속할 때는 서로 다른 포트를 사용합니다. 아래 명령은 별도 터미널에서 각각 실행합니다.

```bash
kubectl --context cluster-a -n k-watch port-forward svc/k-watch 8282:8282
kubectl --context cluster-b -n k-watch port-forward svc/k-watch 8283:8282
```

북마크는 브라우저의 localStorage에 저장되므로 브라우저와 접속 주소별로 관리됩니다.

## 설정

전체 옵션과 기본값은 [values.yaml](charts/k-watch/values.yaml)을 참고하세요.

기본 설치는 `ClusterIP` Service의 `8282` 포트를 사용하며 Ingress는 비활성화되어 있습니다. 컨테이너 리소스 요청은 CPU `100m`·메모리 `128Mi`, 제한은 CPU `500m`·메모리 `512Mi`입니다. `nodeSelector`, `tolerations`, `affinity`, `imagePullSecrets`도 지정할 수 있습니다.

### Ingress로 접속하기

먼저 클러스터의 IngressClass와 인증 구성을 확인한 뒤 values 파일을 작성합니다. K-Watch 자체에는 로그인 기능이 없으므로 Ingress 컨트롤러의 인증 기능이나 인증 프록시 등을 함께 구성하세요.

```yaml
# my-values.yaml
ingress:
  enabled: true
  className: traefik # 클러스터의 IngressClass 이름으로 변경
  hosts:
    - host: k-watch.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: k-watch-tls # 미리 준비한 TLS Secret
      hosts:
        - k-watch.example.com
```

```bash
helm upgrade --install k-watch oci://ghcr.io/klolarion/charts/k-watch \
  --version 0.1.0 \
  --namespace k-watch \
  --create-namespace \
  -f my-values.yaml
```

Ingress를 통과하는 실시간 Pod 스트림에는 SSE 연결이 필요합니다. 프록시의 응답 버퍼링과 연결 제한 시간도 확인하세요.

### 삭제

```bash
helm uninstall k-watch --namespace k-watch
```

차트가 생성한 ClusterRole과 ClusterRoleBinding도 함께 삭제됩니다. `k-watch` 네임스페이스는 남습니다.

## 권한과 접근 제어

K-Watch의 기본 Helm 배포는 클러스터 전체에 대한 **읽기 전용 ServiceAccount**를 사용합니다.

- **조회 대상** — Pod, Service, Namespace, Node, PV, PVC, Event, Deployment, ReplicaSet, StatefulSet, DaemonSet, Job, CronJob, Ingress, EndpointSlice와 노드·Pod 메트릭
- **허용 작업** — 일반 리소스는 `get`, `list`, `watch`, 메트릭은 `get`, `list`
- **권한에서 제외** — Secret, ConfigMap, 리소스 생성·수정·삭제, Pod exec 및 로그 조회
- **기본 컨테이너 설정** — UID/GID `65532`, 읽기 전용 루트 파일시스템, 권한 상승 차단, Linux capability 전체 제거, `RuntimeDefault` seccomp

정확한 권한은 [rbac.yaml](charts/k-watch/templates/rbac.yaml), 컨테이너 설정은 [values.yaml](charts/k-watch/values.yaml)에 있습니다.

**K-Watch에 접속할 수 있는 사람은 이 ServiceAccount가 조회하는 클러스터 정보를 볼 수 있습니다.** 네임스페이스 선택은 화면의 조회 범위를 정하는 기능이며 사용자별 권한 분리는 제공하지 않습니다. 기본 Service는 ClusterIP이므로 클러스터 네트워크에서도 접근할 수 있습니다. 로컬에서는 `kubectl port-forward`를 사용하고, 공유 주소로 제공할 때는 인증 프록시·Ingress 인증·VPN 등으로 접근을 관리하세요.

## 문제 해결

### 화면이 열리지 않아요

Pod가 준비되었는지, 포트 포워딩이 실행 중인지 확인합니다.

```bash
kubectl -n k-watch get pods
kubectl -n k-watch logs deployment/k-watch
```

로컬 `8282` 포트를 이미 사용 중이라면 `8283:8282`로 포워딩하고 `http://localhost:8283`에 접속하세요.

### CPU·메모리 사용률이 표시되지 않아요

metrics-server가 설치되어 있고 메트릭을 제공하는지 확인합니다.

```bash
kubectl top nodes
```

메트릭이 없어도 리소스 지도와 Pod 이벤트는 사용할 수 있습니다.

### 목록 조회에 실패하거나 Live가 Stopped로 표시돼요

화면의 오류와 백엔드 로그를 확인하세요. `Forbidden` 오류라면 ServiceAccount의 조회·Watch 권한을 확인합니다. 예를 들어 기본 설치의 Pod Watch 권한은 다음처럼 확인할 수 있습니다. 이 명령을 실행하는 사용자에게는 해당 ServiceAccount를 impersonate할 권한이 필요합니다.

```bash
kubectl auth can-i watch pods --all-namespaces \
  --as=system:serviceaccount:k-watch:k-watch
```

권한이나 연결 문제를 해결한 뒤 상단 새로고침 버튼으로 다시 연결합니다. Ingress에서만 이벤트가 지연되거나 끊기면 프록시의 SSE 설정도 확인하세요.

## 개발

Go(Gin, client-go) 백엔드가 Kubernetes API를 조회하고 React·TypeScript 프런트엔드를 함께 제공합니다. 지도는 Cytoscape.js, Overview 차트는 Recharts를 사용합니다. 배포 시 별도 프런트엔드 서버는 필요하지 않습니다.

```text
backend/         API, 리소스 관계 구성, Pod Watch → SSE
frontend/        Overview, 네임스페이스 지도, 상세 패널
charts/k-watch/  Helm 차트와 읽기 전용 RBAC
dev/             로컬 실행 안내와 테스트용 매니페스트
scripts/         릴리스 버전 일관성 검사
```

로컬 개발에는 Go 1.26 이상과 Node.js 22가 필요합니다. 실행 방법, 검증 명령, Minikube 배포와 데모 리소스는 **[개발 가이드](dev/README.md)**를 참고하세요.

### 릴리스

1. [Chart.yaml](charts/k-watch/Chart.yaml)의 `version`, `appVersion`, `images` 주석과 [README.md](README.md), [README.ko.md](README.ko.md)의 `--version` 예시를 새 버전으로 맞춥니다.
2. `scripts/check-version.sh <version>`으로 버전 일관성을 확인합니다.
3. `v0.1.0` 형식의 태그를 push합니다. [릴리스 워크플로](.github/workflows/release.yml)가 버전을 검증하고 멀티 아키텍처 이미지와 OCI Helm 차트를 GHCR에 게시합니다.
4. 첫 릴리스 뒤 GHCR의 `k-watch`, `charts/k-watch` 패키지를 Public으로 설정합니다.

## License

[Apache License 2.0](LICENSE)
