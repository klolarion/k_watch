# 개발

설치해서 쓰는 방법은 [README](../README.md)에 있습니다. 이 문서는 코드를 고치고 확인하는 방법입니다.

## 로컬에서 실행

kubectl이 가리키는 클러스터(`~/.kube/config`의 현재 컨텍스트)를 읽습니다. Go 1.26 이상과 Node.js 22가 필요합니다.

```bash
# 터미널 1: API (http://localhost:8282)
cd backend
go run .

# 터미널 2: 화면 (http://localhost:5173)
cd frontend
npm ci
npm run dev
```

화면은 http://localhost:5173 에서 봅니다. Vite가 `/api` 요청을 8282로 넘깁니다.

## 테스트

```bash
cd backend && go vet ./... && go test ./...
cd frontend && npm run lint && npm run build
```

CI(`.github/workflows/ci.yml`)도 같은 것을 돌리고, 차트 lint와 버전 표기 검사(`scripts/check-version.sh`)를 더 합니다.

## 클러스터에 직접 올려 보기

`k8s-manifest.yaml`은 Helm 없이 로컬 이미지를 Minikube에 올리는 매니페스트입니다. 보안 설정은 차트 기본값과 같습니다.

```bash
# 저장소 루트에서
docker build -t k-watch:latest .
minikube image load k-watch:latest

kubectl apply -f dev/k8s-manifest.yaml
minikube service k-watch -n k-watch
```

Minikube가 아니면 마지막 줄 대신 `kubectl -n k-watch port-forward svc/k-watch 8282:8282`로 엽니다. 이때 `imagePullPolicy: Never`이므로 이미지는 그 클러스터의 노드에 미리 들어가 있어야 합니다.

## 지도에 그릴 예제 리소스

`demo.yaml`은 `team-a`, `team-b`, `team-c` 네임스페이스에 Deployment, StatefulSet, DaemonSet, Service, Ingress, PVC를 만들고, `default` 네임스페이스에 Job과 CronJob을 하나씩 만듭니다. PV는 노드의 `/tmp` 아래 hostPath를 쓰므로 Minikube 같은 단일 노드 테스트 클러스터에서만 쓰십시오.

```bash
kubectl apply -f dev/demo.yaml
```

## 정리

```bash
kubectl delete -f dev/demo.yaml
kubectl delete -f dev/k8s-manifest.yaml
```
