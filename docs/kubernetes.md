# Kubernetes: conceitos fundamentais

## O que é Kubernetes?

Kubernetes (também chamado de **k8s** — "k" + 8 letras + "s") é uma
plataforma open-source de **orquestração de containers**. Criada
originalmente pelo Google e hoje mantida pela CNCF (Cloud Native Computing
Foundation), ela resolve os problemas que aparecem quando se roda muitos
containers em produção:

- **Self-healing**: se um container/Pod morre, o Kubernetes recria
  automaticamente.
- **Scaling**: aumenta ou diminui o número de réplicas de uma aplicação
  conforme a demanda (manual ou automaticamente).
- **Service discovery e balanceamento de carga**: dá um nome/DNS estável
  para um grupo de Pods e distribui o tráfego entre eles.
- **Rollouts e rollbacks**: atualiza a aplicação gradualmente e permite
  voltar para a versão anterior se algo der errado.
- **Gerenciamento de configuração e segredos**, agendamento de onde cada
  container deve rodar, etc.

Em resumo: você descreve o **estado desejado** (ex.: "quero 10 réplicas da
imagem `thomasmoraes02/hello-go` rodando") em arquivos YAML, e o Kubernetes
trabalha continuamente para manter a realidade igual a essa descrição.

## Cluster

Um **cluster** é o conjunto de máquinas (físicas, virtuais ou, no caso do
`kind`, containers Docker) que juntas executam o Kubernetes e as
aplicações. Um cluster é dividido em duas partes:

### Control Plane (o "cérebro")

Componentes que tomam as decisões sobre o cluster:

- `kube-apiserver`: porta de entrada — todo comando `kubectl` fala com ele.
- `etcd`: banco de dados que guarda o estado do cluster.
- `kube-scheduler`: decide em qual node cada novo Pod deve rodar.
- `kube-controller-manager`: roda os "controllers" que garantem o estado
  desejado (ex.: recriar um Pod que morreu).

### Data Plane (os "braços")

Os **worker nodes**, que efetivamente executam os containers das
aplicações.

## Nodes

Um **node** é uma máquina (VM, servidor físico ou, no `kind`, um container
Docker) que faz parte do cluster. Cada node roda:

- `kubelet`: agente que conversa com o control plane e garante que os
  containers descritos nos Pods atribuídos a esse node estejam rodando.
- Um **container runtime** (containerd, CRI-O, etc.) que efetivamente cria
  e executa os containers.
- `kube-proxy`: cuida das regras de rede que permitem que os Services
  encontrem os Pods.

Existem dois tipos de node:

- **Control-plane node**: roda os componentes do control plane (em clusters
  pequenos/dev, como o do `kind`, pode ser um único node fazendo os dois
  papéis).
- **Worker node**: roda somente as aplicações (Pods).

## Onde este projeto se encaixa

Ao rodar `kind create cluster --name fullcycle`, o `kind` cria um cluster
Kubernetes completo (control plane + worker) usando **containers Docker
como se fossem nodes**, tudo dentro da sua máquina. Veja
[kind.md](kind.md) para os detalhes.
