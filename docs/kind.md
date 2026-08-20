# kind (Kubernetes IN Docker)

**kind** é uma ferramenta que cria clusters Kubernetes **locais**, usando
containers Docker para simular os nodes do cluster (cada "node" é, na
prática, um container Docker rodando um Kubernetes completo dentro dele).
É muito usada para desenvolvimento local e CI, porque sobe um cluster real
em segundos, sem precisar de VMs ou de um provedor de nuvem.

## Instalação

```bash
brew install kind
kind --version
```

Pré-requisito: o Docker precisa estar instalado e rodando, já que os nodes
do kind são containers Docker.

## Criando um cluster

```bash
kind create cluster --name fullcycle
```

Isso cria (por padrão) um único container fazendo o papel de control-plane
node, sobe todos os componentes do Kubernetes dentro dele, e já configura o
`kubectl` (via `~/.kube/config`) para apontar para esse cluster.

## Outros comandos do kind

```bash
kind get clusters                       # lista os clusters kind existentes
kubectl cluster-info --context kind-fullcycle  # informações do cluster
kind delete cluster --name fullcycle     # apaga o cluster (e os containers)
```

## Carregando uma imagem local no kind

Este é o passo que costuma confundir: como os nodes do kind são containers
Docker **isolados** do Docker do seu host, rodar `docker build` na sua
máquina **não** torna a imagem automaticamente visível dentro do cluster.
Se você aplicar um manifest apontando para essa imagem, o Kubernetes vai
tentar baixá-la de um registry (Docker Hub) e, se ela não tiver sido
publicada lá, o Pod fica em `ErrImagePull` / `ImagePullBackOff`.

Existem duas formas de resolver isso:

### Opção 1 — carregar a imagem diretamente no cluster kind (recomendado para dev)

```bash
docker build -t thomasmoraes02/hello-go .
kind load docker-image thomasmoraes02/hello-go:latest --name fullcycle
```

Isso copia a imagem do Docker do host para dentro de todos os nodes do
cluster kind, sem precisar de nenhum registry externo.

> ⚠️ Atenção ao `imagePullPolicy`: quando a tag da imagem é `latest` (como
> em `k8s/deployment.yaml` e `k8s/pod.yaml`), o Kubernetes usa por padrão
> `imagePullPolicy: Always`, e tenta **sempre** baixar a imagem de um
> registry, ignorando a imagem carregada localmente com `kind load`. Para
> garantir que o cluster use a imagem carregada localmente, adicione
> `imagePullPolicy: IfNotPresent` (ou `Never`) ao container no manifest.

### Opção 2 — publicar a imagem em um registry (Docker Hub)

```bash
docker build -t thomasmoraes02/hello-go .
docker push thomasmoraes02/hello-go
```

Com a imagem publicada, qualquer cluster Kubernetes (kind, EKS, GKE, etc.)
consegue baixá-la normalmente, sem passos extras — é a abordagem usada em
produção.

## Depois de carregar a imagem

Com a imagem disponível no cluster, siga o fluxo normal do Kubernetes: veja
[kubectl.md](kubectl.md) para aplicar os manifests, e
[services.md](services.md#acessando-o-service-no-kind) para acessar a
aplicação rodando dentro do kind.
