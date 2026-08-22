# Lens (Kubernetes IDE)

**Lens** é uma aplicação desktop (macOS/Windows/Linux) que funciona como uma
IDE gráfica para Kubernetes: lê o mesmo `~/.kube/config` que o `kubectl`
usa e te dá uma interface visual para navegar Pods, Deployments, Services,
logs, métricas, terminal e Helm — sem precisar decorar comando por comando.
Não substitui o `kubectl` (por baixo dos panos ele faz as mesmas chamadas
à API do Kubernetes), mas acelera bastante o dia a dia, principalmente em
clusters com muitos recursos, como costuma ser em produção (ex.: EKS no
trabalho).

## Instalação (macOS)

```bash
brew install --cask lens
```

Isso instala `Lens.app` em `/Applications`. Abra pelo Spotlight/Launchpad
como qualquer app. Alternativa: baixar o `.dmg` direto em
[k8slens.dev](https://k8slens.dev/).

## Conceitos da interface

- **Catalog**: a tela inicial — lista todos os clusters conhecidos (um por
  contexto do seu `~/.kube/config`). É daqui que você abre uma conexão.
- **Hotbar**: a barra lateral esquerda com ícones dos clusters "fixados",
  pra trocar rápido entre eles sem voltar ao Catalog.
- **Cluster view**: depois de clicar em um cluster, é a tela principal —
  menu à esquerda com os tipos de recurso (Workloads, Config, Network,
  Storage, Namespaces, Nodes, Helm, etc.), conteúdo no centro.
- **Namespace selector**: no topo da Cluster view — filtra o que é
  exibido. Por padrão mostra todos os namespaces; em clusters de produção
  grandes (como um EKS compartilhado), vale filtrar para o(s) namespace(s)
  do seu time, senão a lista fica poluída com recursos de outros times.

## Conectando ao cluster local deste projeto (kind)

Já coberto quando instalamos: o Lens lê `~/.kube/config` automaticamente,
então o contexto `kind-fullcycle` (criado por `kind create cluster
--name fullcycle`, ver [docs/kind.md](kind.md)) já aparece no Catalog sem
nenhuma configuração extra. Clique nele para conectar.

## Conectando a um cluster AWS EKS (uso no trabalho)

Diferente do `kind` (que configura o kubeconfig sozinho), um cluster EKS
exige autenticação via IAM. O Lens **não fala com a AWS diretamente** — ele
só usa o kubeconfig que você gerou com a AWS CLI, exatamente como o
`kubectl` faria.

### Pré-requisitos

```bash
brew install awscli
aws --version        # confirme AWS CLI v2
```

Credenciais configuradas — depende de como seu time faz login:

```bash
# Se o time usa AWS SSO (mais comum hoje em empresas)
aws configure sso
aws sso login --profile <nome-do-profile>

# Se o time usa access key/secret tradicional
aws configure --profile <nome-do-profile>
```

### Gerando o kubeconfig do cluster EKS

```bash
aws eks update-kubeconfig \
  --region <região-do-cluster> \
  --name <nome-do-cluster-eks> \
  --profile <nome-do-profile>
```

Isso **adiciona** um novo contexto ao seu `~/.kube/config` (não sobrescreve
os existentes, como o `kind-fullcycle`) — um bloco `exec` que chama
`aws eks get-token` para gerar um token IAM de curta duração toda vez que
o `kubectl` (ou o Lens) precisa autenticar. Verifique antes de abrir o
Lens:

```bash
kubectl config get-contexts                 # confirme que o contexto novo apareceu
kubectl config use-context <contexto-eks>
kubectl get nodes                            # se isso funcionar, o Lens também vai funcionar
```

### Abrindo no Lens

Com o `kubectl get nodes` funcionando, é só abrir o Lens — o contexto novo
aparece no **Catalog** junto com os outros, sem precisar reconfigurar nada
dentro do app.

### Erros comuns em EKS

- **`error: You must be logged in to the server (Unauthorized)`**: sua
  identidade IAM (usuário/role) não está mapeada no controle de acesso do
  cluster. Em clusters mais antigos, isso é o `ConfigMap aws-auth` no
  namespace `kube-system`; em clusters mais novos, pode ser gerenciado via
  **EKS Access Entries** (API da AWS, fora do cluster). Nos dois casos,
  quem administra o cluster precisa te adicionar — não é algo que se
  resolve só localmente.
- **Token expirado / sessão SSO expirada**: como a autenticação é via
  `exec` (token de curta duração), depois de um tempo parado o Lens volta
  a mostrar erro de conexão. Rode `aws sso login --profile <profile>` de
  novo no terminal, e reconecte o cluster no Lens (clique nele no
  Catalog/Hotbar de novo).
- **Profile errado sendo usado**: se você tem múltiplos profiles da AWS,
  confira qual foi usado no `aws eks update-kubeconfig --profile ...` —
  o Lens só reflete o que está no kubeconfig, ele não deixa escolher
  profile por dentro do app.
- **Cluster não aparece na lista**: geralmente é porque o
  `~/.kube/config` não foi atualizado (esqueceu o `aws eks
  update-kubeconfig`) ou porque o Lens estava aberto quando você rodou o
  comando — feche e abra de novo, ou use "Add Cluster" manualmente
  apontando para o kubeconfig.

## Principais funcionalidades

### Workloads

Pods, Deployments, ReplicaSets, StatefulSets, DaemonSets, Jobs, CronJobs —
listados com status (`Running`, `CrashLoopBackOff`, etc.), réplicas
prontas, idade. Clicar em um item abre o detalhe (o YAML completo, eventos,
containers). É o equivalente visual de `kubectl get`/`kubectl describe`.

### Terminal integrado (equivalente a `kubectl exec`)

Dentro do detalhe de um Pod, o ícone de terminal abre um shell **direto
dentro do container**, sem precisar digitar `kubectl exec -it <pod> -- sh`
manualmente nem descobrir o nome exato do Pod primeiro (útil quando o nome
tem hash aleatório, como `goserver-64657dcc68-55qbl`).

### Logs

Ícone de logs no detalhe do Pod — stream ao vivo, com busca por texto,
sem precisar rodar `kubectl logs -f <pod>` em um terminal separado. Dá
pra ver logs de um container específico se o Pod tiver mais de um.

### Port-forward pela UI

Botão de port-forward no detalhe de um Pod/Service — equivalente a
`kubectl port-forward svc/goserver-service 8080:80`
([docs/services.md](services.md#acessando-o-service-no-kind)), mas sem
precisar manter um terminal aberto rodando o comando.

### Editar recursos (YAML inline)

Cada recurso tem uma aba "Edit" com o YAML completo — editar e salvar
aplica a mudança no cluster (equivalente a `kubectl edit` ou a um `kubectl
apply` do YAML editado). Útil para ajustes rápidos e pontuais; para
mudanças permanentes, o correto continua sendo editar o manifest no
repositório e reaplicar (senão a próxima pessoa que rodar `kubectl apply
-f` a partir do Git desfaz sua edição manual — o mesmo alerta que já vale
para `kubectl edit` puro).

### Métricas e gráficos

Gráficos de uso de CPU/memória por Pod/node, direto na interface —
equivalente visual de `kubectl top pod`/`kubectl top nodes`
([docs/autoscaling.md](autoscaling.md#kubectl-top-pod-vendo-o-uso-real-de-um-pod)).
**Depende do `metrics-server` estar instalado no cluster** — sem ele, os
gráficos ficam vazios, exatamente pelo mesmo motivo que `kubectl top`
falha sem ele. Em clusters EKS geridos por outro time, o `metrics-server`
geralmente já vem instalado; neste projeto, foi instalado manualmente
(ver [docs/autoscaling.md](autoscaling.md#pré-requisito-metrics-server)).

### Namespaces e Nodes

Visão de todos os `Namespace`s do cluster e de todos os `Node`s
(capacidade total vs alocada, quantos Pods rodando em cada um) — útil pra
entender rapidamente se um cluster compartilhado (como um EKS de time)
está com pouco espaço para novos Pods.

### Config e Storage

`ConfigMap`s e `Secret`s (Config), `PersistentVolume`/`PersistentVolumeClaim`/`StorageClass`
(Storage) — navegáveis e editáveis do mesmo jeito que Workloads.
⚠️ Secrets aparecem com os valores em base64 decodificados na UI — tome
cuidado ao compartilhar tela em produção.

### Network

`Service`s, `Ingress`, `Endpoints`, `NetworkPolicy` — mostra a quais Pods
um Service está realmente roteando (o equivalente visual de `kubectl get
endpoints`, já mencionado em [docs/services.md](services.md)).

### Helm

Se o cluster usa [Helm](https://helm.sh/), o Lens lista Releases
instaladas e o Repository de Charts configurado, com opção de instalar/
atualizar/remover releases pela UI — não é usado neste projeto (que não
tem Helm charts), mas é comum em ambientes de produção como EKS.

## Lens vs kubectl — equivalências rápidas

| Ação | `kubectl` | Lens |
|---|---|---|
| Listar Pods | `kubectl get pods` | Workloads → Pods |
| Ver detalhes/eventos | `kubectl describe pod <nome>` | Clicar no Pod |
| Ver logs | `kubectl logs -f <nome>` | Ícone de logs no detalhe do Pod |
| Entrar no container | `kubectl exec -it <nome> -- sh` | Ícone de terminal no detalhe do Pod |
| Editar um recurso | `kubectl edit <tipo> <nome>` | Aba "Edit" no detalhe do recurso |
| Port-forward | `kubectl port-forward svc/<nome> 8080:80` | Botão de port-forward |
| Uso de CPU/memória | `kubectl top pod` / `kubectl top nodes` | Aba de métricas/gráficos (precisa de metrics-server) |
| Trocar de cluster | `kubectl config use-context <ctx>` | Clicar no cluster no Catalog/Hotbar |
| Ver HPA escalando | `kubectl get hpa -w` | Workloads → Horizontal Pod Autoscalers |

## Dicas

- **Fixe no Hotbar** os clusters que você usa todo dia (clique direito no
  cluster → "Pin to Hotbar") — evita voltar ao Catalog toda vez.
- **Filtro de namespace** é o primeiro lugar a checar quando "um recurso
  que eu sei que existe não aparece na lista" — provavelmente está fora do
  namespace selecionado no topo.
- **Múltiplos clusters ao mesmo tempo**: dá pra ter o `kind-fullcycle`
  local e um cluster EKS de produção abertos em abas separadas — preste
  atenção em qual aba está ativa antes de editar/deletar algo, para não
  aplicar uma mudança de teste no cluster errado.
- **Extensões**: o Lens suporta extensões (ex.: integração com
  Prometheus para métricas mais avançadas que o `metrics-server` básico
  oferece) — vale olhar o marketplace de extensões dentro do app se
  precisar de mais do que o que vem por padrão.

## Troubleshooting comum

| Sintoma | Causa provável | Solução |
|---|---|---|
| Cluster não aparece no Catalog | `~/.kube/config` não tem o contexto, ou o Lens estava aberto durante a mudança | Confirme com `kubectl config get-contexts`; feche e reabra o Lens |
| "Unauthorized" ao conectar (EKS) | Identidade IAM não mapeada no cluster, ou sessão SSO expirada | Peça mapeamento a quem administra o cluster; rode `aws sso login` de novo |
| Métricas/gráficos vazios | `metrics-server` não instalado no cluster | Confirme com `kubectl top nodes`; se falhar também, o problema é o cluster, não o Lens (ver [docs/autoscaling.md](autoscaling.md#pré-requisito-metrics-server)) |
| Pod que eu sei que existe não aparece | Filtro de namespace errado | Mude o namespace selecionado no topo para "All namespaces" |
| Editei um recurso pelo Lens e ele "voltou" sozinho | Algum controlador (Deployment, HPA, GitOps como ArgoCD/Flux) reconciliou o estado de volta ao declarado | Edite a fonte real (o manifest no Git / o Deployment), não o recurso final gerado |

## Referências

- [Site oficial do Lens](https://k8slens.dev/)
- [Documentação da AWS sobre `aws eks update-kubeconfig`](https://docs.aws.amazon.com/cli/latest/reference/eks/update-kubeconfig.html)
