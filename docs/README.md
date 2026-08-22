# Projeto Kubernetes — Full Cycle

Servidor HTTP simples em Go ("Hello Full Cycle!"), containerizado com Docker e
implantado em um cluster Kubernetes local criado com **kind**. Este projeto
serve para praticar os conceitos básicos de Kubernetes: Pod, Deployment,
Service e o fluxo de build/deploy de uma imagem local.

## Estrutura do projeto

```
kubernetes/
├── server.go        # aplicação Go: responde "Hello Full Cycle!" na porta 80
├── Dockerfile        # build da imagem golang:1.15-alpine
├── k8s/
│   ├── pod.yaml        # Pod isolado (uso didático)
│   ├── deployment.yaml # Deployment com 10 réplicas
│   └── service.yaml    # Service tipo LoadBalancer
└── docs/              # esta documentação
    ├── kubernetes.md   # conceitos: cluster, nodes, o que é k8s
    ├── pods.md         # conceito de Pod
    ├── deployments.md  # conceito de Deployment
    ├── probes.md       # liveness e readiness probes
    ├── services.md     # conceito de Service
    ├── kind.md         # criar cluster local e carregar imagens
    ├── kubectl.md       # comandos do kubectl
    └── golang.md        # build/run da imagem Docker
```

## Fluxo completo (do zero até acessar o app)

1. **Build da imagem Docker** — ver [golang.md](golang.md)
   ```bash
   docker build -t thomasmoraes02/hello-go .
   ```
2. **Criar o cluster local com kind** — ver [kind.md](kind.md)
   ```bash
   kind create cluster --name fullcycle
   ```
3. **Levar a imagem local para dentro do cluster kind** — ver [kind.md](kind.md#carregando-uma-imagem-local-no-kind)
   ```bash
   kind load docker-image thomasmoraes02/hello-go:latest --name fullcycle
   ```
4. **Aplicar os manifests** — ver [kubectl.md](kubectl.md)
   ```bash
   kubectl apply -f k8s/deployment.yaml
   kubectl apply -f k8s/service.yaml
   ```
5. **Acessar a aplicação** — ver [services.md](services.md#acessando-o-service-no-kind)
   ```bash
   kubectl port-forward svc/goserver-service 8080:80
   curl localhost:8080
   ```

> ⚠️ Ao documentar os manifests foram encontradas duas inconsistências de porta
> entre `server.go` (escuta em `:80`), `deployment.yaml`
> (`containerPort: 8080`) e `service.yaml` (`targetPort: 8000`). Isso é
> explicado em detalhe em [deployments.md](deployments.md#observação-sobre-a-porta)
> e [services.md](services.md#observação-sobre-a-porta), pois afeta se o
> `port-forward`/Service realmente consegue falar com o processo Go.

## Leitura recomendada (ordem)

1. [kubernetes.md](kubernetes.md) — o que é Kubernetes, cluster e nodes
2. [pods.md](pods.md) — a menor unidade do Kubernetes
3. [deployments.md](deployments.md) — gerenciando réplicas de Pods
4. [probes.md](probes.md) — liveness e readiness probes
5. [services.md](services.md) — expondo os Pods na rede
6. [kind.md](kind.md) — rodando um cluster localmente com Docker
7. [kubectl.md](kubectl.md) — comandos do dia a dia
8. [golang.md](golang.md) — build e execução da imagem da aplicação
