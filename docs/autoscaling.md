# Resources, Limits e HPA (Horizontal Pod Autoscaler)

Autoscaling no Kubernetes só funciona se o Kubernetes souber **quanto**
recurso cada Pod consome e **quanto** ele tem disponível para consumir. Por
isso este doc começa em `resources` (requests/limits) — é a base sobre a
qual o HPA calcula tudo.

## `resources`: requests e limits

```yaml
resources:
  requests:
    cpu: "0.05"
    memory: "20Mi"
  limits:
    cpu: "0.05"
    memory: "25Mi"
```

(bloco real do [k8s/deployment.yaml](../k8s/deployment.yaml))

- **`requests`**: quanto o container **reserva** garantidamente. O
  `kube-scheduler` só coloca o Pod em um node que tenha essa quantidade
  livre. É também a base usada para calcular **porcentagem de uso** (ex.:
  em métricas de CPU do HPA, ver abaixo).
- **`limits`**: o **teto** que o container pode consumir.
  - Ultrapassar o `limits.cpu` **não mata o container** — o kernel apenas
    faz *throttling* (a aplicação fica mais lenta, "engasgando").
  - Ultrapassar o `limits.memory` **mata o container** imediatamente
    (`OOMKilled`), pois memória não pode ser "throttled" como CPU.
- Unidades: CPU em **millicores** (`1000m` = 1 vCPU inteira) **ou** em
  fração de vCPU (`"0.05"` é exatamente equivalente a `"50m"` — este
  projeto usa a notação fracionária); memória em `Mi` (mebibytes) ou `Gi`
  (gibibytes).
- Aqui `requests.cpu` e `limits.cpu` são **iguais** (`0.05` = `0.05`), mas
  `requests.memory` (`20Mi`) e `limits.memory` (`25Mi`) **não são**. Isso
  define a **QoS Class** do Pod: só é `Guaranteed` quando request == limit
  em **todos** os recursos, de **todos** os containers; bastando um
  recurso divergir (aqui, a memória), o Pod cai para `Burstable`. Confira
  com `kubectl get pod <nome> -o jsonpath='{.status.qosClass}'`.
- Sem `requests` definido, o scheduler não sabe quanto reservar (pode
  empilhar Pods demais em um node) e **o HPA baseado em CPU não funciona**,
  porque a porcentagem de utilização é sempre calculada em relação ao
  `requests.cpu`, não ao `limits.cpu`.

## O que é o HPA

O **HorizontalPodAutoscaler** observa uma métrica (normalmente % de uso de
CPU) e ajusta automaticamente o campo `replicas` de um Deployment — criando
mais Pods sob carga alta, e removendo Pods quando a carga cai. É
"horizontal" porque escala **quantidade de Pods**, não o tamanho de cada
um (isso seria escala **vertical**, feita por outro objeto, o
`VerticalPodAutoscaler` — não usado neste projeto).

| Tipo de autoscaling | O que muda | Objeto |
|---|---|---|
| **Horizontal** (HPA) | Número de réplicas (Pods) | `HorizontalPodAutoscaler` |
| Vertical (VPA) | `resources.requests`/`limits` de cada Pod | `VerticalPodAutoscaler` (add-on separado) |
| Cluster Autoscaler | Número de **nodes** do cluster | Específico do provedor de nuvem — não existe em `kind` |

## Como o HPA calcula quantas réplicas manter

Fórmula (simplificada, para métrica de CPU):

```
réplicas desejadas = ceil( réplicas atuais × (uso atual / uso alvo) )
```

Exemplo com o `requests.cpu: 0.05` (`50m`) deste projeto e o alvo real de
25% do `k8s/hpa.yaml`:

- Uso alvo = 25% de `50m` = `12,5m` por Pod.
- Se a **média** de uso entre os Pods está em `25m` (50% de utilização):
  `réplicas desejadas = ceil(1 × (25/12,5)) = 2` → o HPA sobe para 2
  réplicas.
- Se depois a média cai para `6m` (~12% de utilização):
  `ceil(2 × (6/12,5)) = 1` → volta para 1 réplica.

Ou seja: **é sempre uma porcentagem do `requests`, não do `limits`.**

## O HPA deste projeto (`k8s/hpa.yaml`)

```yaml
apiVersion: autoscaling/v1
kind: HorizontalPodAutoscaler
metadata:
  name: goserver-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    name: goserver
    kind: Deployment
  # Escala no máximo 5 replicas
  minReplicas: 1
  maxReplicas: 30
  targetCPUUtilizationPercentage: 25
```

- `scaleTargetRef`: aponta para o Deployment que o HPA vai controlar
  (`goserver`, o mesmo do [deployments.md](deployments.md)).
- `minReplicas` / `maxReplicas`: piso e teto de réplicas — o HPA nunca sai
  desse intervalo (aqui, entre 1 e 30), mesmo em picos extremos de carga.
  ⚠️ Repare que o comentário no próprio arquivo (`# Escala no máximo 5
  replicas`) ficou **desatualizado** — o valor real de `maxReplicas` é
  `30`. Comentários em YAML não são validados por nada, então é fácil
  ficarem defasados depois de um ajuste; vale corrigir o texto do
  comentário para não confundir quem ler o manifest depois.
- `targetCPUUtilizationPercentage: 25`: mantém a **média** de uso de CPU
  entre os Pods em torno de 25% do `requests.cpu` (`50m` → alvo de
  `12,5m` por Pod) — um alvo bem sensível, então esse HPA escala com uma
  carga relativamente baixa.

Aplique com:

```bash
kubectl apply -f k8s/hpa.yaml
```

### `autoscaling/v1` vs `autoscaling/v2`

O manifest real deste projeto usa a API **`v1`**, mais simples e mais
antiga: só suporta escalar por CPU, direto no campo
`targetCPUUtilizationPercentage`. A API **`v2`** (usada no exemplo de seção
anterior) é a versão atual e mais flexível: permite múltiplas métricas ao
mesmo tempo (CPU, memória, métricas customizadas), uma por item da lista
`metrics[]`, além de `behavior` para controlar a velocidade de scale
up/down. Para escalar só por CPU (que é o caso deste projeto),
`autoscaling/v1` já resolve; `v2` só vale a pena quando você precisa de
mais de uma métrica.

## Pré-requisito: `metrics-server`

O HPA baseado em CPU/memória depende do **metrics-server** rodando no
cluster (ele é quem expõe `kubectl top` e alimenta o HPA). Cluster `kind`
**não vem com ele por padrão**.

### Como foi instalado neste projeto (`k8s/metrics-server.yaml`)

```bash
wget https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
mv components.yaml k8s/metrics-server.yaml
```

O manifest oficial do metrics-server assume um cluster real (com
certificados válidos entre control-plane e kubelet). No `kind`, os
certificados dos kubelets são self-signed, então o metrics-server rejeita a
conexão por padrão. A correção foi editar o arquivo baixado direto e
adicionar a flag `--kubelet-insecure-tls` na lista de `args` do container
`metrics-server` (não via `kubectl patch` — a flag já fica versionada no
próprio manifest):

```yaml
# k8s/metrics-server.yaml (trecho do container metrics-server)
        - --cert-dir=/tmp
        - --secure-port=10250
        - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
        - --kubelet-use-node-status-port
        - --metric-resolution=15s
        - --kubelet-insecure-tls
```

- `--kubelet-insecure-tls`: pula a verificação do certificado TLS do
  kubelet. **Aceitável em `kind`/desenvolvimento local**, mas não deveria
  ser usado em produção — lá o certificado do kubelet deve ser válido e
  verificado normalmente.
- `--metric-resolution=15s`: de quanto em quanto tempo o metrics-server
  coleta métricas frescas dos kubelets (o padrão do binário já vem como
  `15s` neste manifest).

Depois de editar, aplique como qualquer outro manifest:

```bash
kubectl apply -f k8s/metrics-server.yaml
```

### Confirmando que está funcionando

```bash
$ kubectl get deployment metrics-server -n kube-system
NAME             READY   UP-TO-DATE   AVAILABLE   AGE
metrics-server   1/1     1            1           96s

$ kubectl top nodes
NAME                      CPU(cores)   CPU(%)   MEMORY(bytes)   MEMORY(%)
fullcycle-control-plane   141m         1%       886Mi           11%
```

`kubectl top nodes` respondeu de primeira. `kubectl top pods`, por outro
lado, pode devolver `error: Metrics not available for pod ...` por um
tempo logo após a instalação — o metrics-server precisa completar pelo
menos um ciclo de coleta (`--metric-resolution`) por Pod antes de ter dado
suficiente para responder; se persistir por mais de um ou dois minutos,
confira `kubectl logs -n kube-system -l k8s-app=metrics-server`.

Sem o `metrics-server`, o HPA fica em `<unknown>` na coluna `TARGETS` de
`kubectl get hpa` e não escala nada.

## `kubectl top pod`: vendo o uso real de um Pod

```bash
$ kubectl top pod goserver-5b45659549-5jpbm
NAME                        CPU(cores)   MEMORY(bytes)
goserver-5b45659549-5jpbm   1m           4Mi
```

Esse comando pergunta ao `metrics-server` (não ao Pod diretamente) o uso
**atual** de CPU/memória, na granularidade do último ciclo de coleta
(`--metric-resolution`, `15s` neste projeto — veja a seção acima). É o
comando mais direto para responder "esse Pod está perto do limite?" ou
"por que o HPA não escalou ainda?".

- `NAME`: o Pod consultado.
- `CPU(cores)`: em millicores, mesma unidade do `resources` do Deployment
  (`1m` = 0,001 vCPU). Compare direto com `requests.cpu: 100m` e
  `limits.cpu: 250m` do [k8s/deployment.yaml](../k8s/deployment.yaml): `1m`
  é **1% do request** — este Pod está praticamente ocioso, bem longe de
  disparar qualquer scale-up.
- `MEMORY(bytes)`: apesar do nome da coluna, vem formatado em `Mi`/`Gi`.
  `4Mi` frente a `requests.memory: 128Mi` também é uso bem baixo (~3%).

### Variações úteis

```bash
kubectl top pod goserver-5b45659549-5jpbm         # um Pod específico, pelo nome
kubectl top pods                                   # todos os Pods do namespace atual
kubectl top pods -l app=goserver                    # só os Pods do Deployment goserver (via label)
kubectl top pods -A                                 # todos os Pods, de todos os namespaces
kubectl top pod goserver-5b45659549-5jpbm --containers
                                                     # detalha uso por container (útil se o Pod tiver mais de um)
kubectl top pods --sort-by=cpu                       # ordena pelo maior consumidor de CPU
kubectl top pods --sort-by=memory                    # ordena pelo maior consumidor de memória
```

### Por que ele "bem importante"

É a forma mais rápida de depurar autoscaling sem esperar o HPA reagir:
compare o número aqui contra o `requests.cpu`/`requests.memory` do
Deployment — se a porcentagem já está perto do `averageUtilization` alvo do
HPA, uma escalada está prestes a acontecer; se está bem abaixo, é esperado
que o HPA não faça nada. É também o primeiro comando a rodar quando
`kubectl get hpa` mostra `<unknown>` nos `TARGETS` — se `kubectl top pod`
falhar, o problema é o `metrics-server`, não o HPA.

## ⚠️ Cuidado: `replicas` fixo no Deployment vs HPA

O [k8s/deployment.yaml](../k8s/deployment.yaml) atual declara `replicas: 1`
explicitamente. Se um HPA estiver ativo controlando esse Deployment, **todo
`kubectl apply -f k8s/deployment.yaml`** reaplica `replicas: 1` e briga com
o HPA (o HPA muda para, digamos, 3 réplicas sob carga, e o próximo `apply`
do Deployment volta para 1). Na prática, quando o HPA está em uso, o campo
`replicas` do Deployment deve ser **omitido** do manifest (deixe o HPA ser a
única fonte de verdade para esse número).

## Gerando carga de verdade com fortio

Ter um HPA aplicado não prova nada sozinho — é preciso gerar carga real
para ver `kubectl get hpa` sair de `<unknown>`/ocioso e realmente escalar.
[fortio](https://github.com/fortio/fortio) é uma ferramenta de load testing
HTTP (criada pela equipe do Istio) que permite controlar precisamente
**taxa de requisições** (QPS) e **concorrência**, o que a torna melhor para
isso do que algo como `ab`/`hey` para simular carga sustentada e previsível.

```bash
kubectl run -it fortio --rm --image=fortio/fortio -- \
  load -qps 800 -t 120s -c 70 "http://goserver-service/healthz"
```

- `kubectl run fortio --image=fortio/fortio`: cria um **Pod avulso**
  (não um Deployment) chamado `fortio`, rodando a imagem oficial do
  fortio — um jeito rápido de rodar uma ferramenta pontual dentro do
  cluster sem escrever um manifest.
- `-it`: `-i` mantém o stdin aberto e `-t` aloca um TTY — junto, conectam
  seu terminal à saída do container, então você acompanha o teste de carga
  rodando em tempo real, como se estivesse na sua própria máquina.
- `--rm`: apaga o Pod automaticamente assim que ele termina (ou quando você
  sai) — evita deixar Pods de teste esquecidos no cluster.
- Tudo depois de `--` é o **comando passado para o container**, sobrescrevendo
  o `CMD` da imagem: aqui é o subcomando `load` do fortio, que dispara o
  teste, com:
  - `-qps 800`: até 800 requisições por segundo, no total (não por
    conexão) — taxa alvo, não um "o mais rápido possível".
  - `-t 120s`: roda por 120 segundos.
  - `-c 70`: 70 conexões concorrentes disparando requisições em paralelo
    para atingir a taxa alvo.
  - `"http://goserver-service/healthz"`: o alvo é o **Service**
    (`goserver-service`, [services.md](services.md)), não um Pod
    diretamente — o DNS interno do cluster resolve esse nome, e o Service
    distribui as 800 req/s entre todos os Pods do `goserver` disponíveis,
    exatamente como uma aplicação real faria.

### ⚠️ `--generator=run-pod/v1` não funciona mais

Se você rodou (ou copiou de um tutorial antigo) o comando com
`--generator=run-pod/v1` no meio, ele vai falhar na versão atual do
`kubectl` (testado aqui: client v1.32):

```
error: unknown flag: --generator
```

Esse flag existia porque, até o `kubectl` 1.17, `kubectl run` conseguia
criar vários tipos de objeto (Pod, Deployment, Job, ReplicationController…)
e `--generator` escolhia qual "gerador" usar — `run-pod/v1` forçava a
criação de um Pod avulso em vez de um Deployment (que era o padrão até
então). Desde o `kubectl` 1.18, esse mecanismo de generators foi
**removido**: `kubectl run` **sempre** cria um Pod avulso agora (é
justamente o que `--generator=run-pod/v1` pedia manualmente antes), então
o flag ficou não só desnecessário como inválido. O comando correto hoje é
o de cima, sem esse flag.

### Por que pode aparecer algum `500` isolado no relatório do fortio

O alvo do teste é a mesma rota `/healthz` documentada em
[probes.md](probes.md#como-isso-se-conecta-com-a-rota-healthz-deste-projeto):
ela simula um boot de `10s` (devolve `500` até lá) e depois fica saudável
**para sempre** — diferente de uma versão anterior do handler (preservada
comentada em [server.go](../server.go#L52-L63) como `HealthzOld`) que
voltava a falhar depois de `30s` e forçava um restart em loop. Essa versão
antiga foi desativada **de propósito** porque atrapalharia justamente este
teste: um Pod sendo derrubado pela `livenessProbe` no meio de um teste de
carga sustentado (120s) distorceria a leitura de CPU do HPA.

Com o handler atual, a única fonte esperada de `500` durante o teste é o
Service mandando tráfego para **Pods novos** ainda nos primeiros 10s de
vida — o que só acontece se o HPA criar réplicas novas no meio do teste
(reagindo à própria carga do fortio) e, mesmo assim, a `readinessProbe`
normalmente já tira esses Pods da rota antes de qualquer requisição
externa chegar até eles. Ou seja: numa execução normal, é esperado ver a
taxa de erro do fortio em `0%` (ou bem próxima disso) — se aparecer um
volume alto de `500`, é sinal de investigar (`kubectl describe pod`,
`kubectl logs`), não um comportamento propositalmente simulado como era
antes.

### Setup completo: 3 terminais

Para ver o HPA reagir em tempo real, o teste roda em três terminais lado a
lado — um gera a carga, os outros dois só observam:

```bash
# Terminal 1 — gera a carga
kubectl run -it fortio --rm --image=fortio/fortio -- load -qps 800 -t 120s -c 70 "http://goserver-service/healthz"

# Terminal 2 — acompanha o HPA decidindo escalar
watch -n1 kubectl get hpa

# Terminal 3 — acompanha os Pods sendo criados/destruídos
watch -n1 kubectl get pods
```

- **Terminal 1 (fortio)**: dispara as 800 req/s por 120s contra o Service
  (ver seção acima) e, ao final, imprime um relatório com percentis de
  latência e contagem de códigos de resposta (`200`, `500`, …).
- **Terminal 2 (`watch -n1 kubectl get hpa`)**: repete `kubectl get hpa` a
  cada 1s. A coluna `TARGETS` mostra `<uso atual>/<uso alvo>` (ex.:
  `35%/25%`) — o CPU médio dos Pods do `goserver`, comparado ao alvo de
  `targetCPUUtilizationPercentage: 25` do [k8s/hpa.yaml](../k8s/hpa.yaml).
  A coluna `REPLICAS` sobe conforme o HPA decide escalar. É o mesmo efeito
  de `kubectl get hpa goserver-hpa -w` (que fica escutando eventos, em vez
  de re-executar o comando do zero a cada segundo) — ambos servem aqui,
  `watch` só é mais familiar se você já usa em outros comandos deste
  projeto (ver a explicação de por que instalar `watch` no início deste
  fluxo).
- **Terminal 3 (`watch -n1 kubectl get pods`)**: mostra a consequência
  prática da decisão do HPA — novos Pods `goserver-xxxxx` aparecendo com
  `READY 0/1` (ainda na janela de `startupProbe`/`readinessProbe`, ver
  [probes.md](probes.md)) até ficarem `1/1` e começarem a receber parte da
  carga do fortio.

### O que esperar ver, na ordem

1. Assim que o fortio começa, os Pods existentes passam a processar 800
   req/s — CPU sobe, e o Terminal 2 mostra `TARGETS` subindo em direção
   (ou passando) de `25%`.
2. Quando a média de CPU fica acima do alvo por tempo suficiente, o HPA
   aumenta `REPLICAS` no Terminal 2, e o Terminal 3 mostra Pods novos
   sendo criados (até o teto `maxReplicas: 5`).
3. Cada Pod novo passa por `READY 0/1` → `1/1` (Terminal 3) antes de
   receber tráfego de verdade — é a `readinessProbe` fazendo seu trabalho
   normalmente, mesmo em plena carga.
4. Quando o fortio termina (120s) e a carga cessa, o CPU cai, e depois de
   alguns minutos (o HPA tem um período de estabilização antes de reduzir,
   pra evitar oscilar réplicas toda hora) o Terminal 2 mostra `REPLICAS`
   voltando a cair até `minReplicas: 1`, e o Terminal 3 mostra os Pods
   extras sendo terminados.

## Comandos úteis

```bash
kubectl top pods                          # uso atual de CPU/memória por Pod (precisa de metrics-server)
kubectl top nodes                         # uso atual por node

kubectl autoscale deployment goserver --cpu-percent=50 --min=1 --max=5
                                            # cria um HPA sem escrever YAML (equivalente ao manifest acima)

kubectl get hpa                            # lista HPAs e mostra TARGETS (uso atual/alvo) e réplicas atuais
kubectl describe hpa goserver-hpa          # eventos de scale up/down e por quê
kubectl delete hpa goserver-hpa            # remove o HPA (Deployment volta a ser controlado manualmente)

kubectl get hpa goserver-hpa -w            # acompanha o HPA reagindo em tempo real
```
