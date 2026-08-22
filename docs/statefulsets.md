# StatefulSet e PersistentVolumeClaim (PVC)

Um **Deployment** trata todas as réplicas como intercambiáveis: qualquer
Pod pode morrer e ser substituído por outro qualquer, sem identidade
própria. Isso não serve para aplicações com estado — um banco de dados
precisa que cada réplica mantenha **o mesmo disco** e **o mesmo endereço de
rede** entre reinícios. É pra isso que existe o **StatefulSet**.

Este projeto usa um StatefulSet de MySQL ([k8s/statefulset.yaml](../k8s/statefulset.yaml))
como laboratório desses conceitos.

## O que o StatefulSet garante, que o Deployment não garante

- **Nomes previsíveis e ordinais**: `mysql-0`, `mysql-1`, `mysql-2`, ...
  (nunca um hash aleatório como nos Pods de um Deployment). O índice é
  estável — se `mysql-1` morrer, o Pod recriado se chama `mysql-1` de
  novo, não um nome novo.
- **Identidade de rede estável**, via um **Service headless** (ver
  abaixo): cada Pod ganha um DNS previsível,
  `<nome-do-pod>.<serviceName>.<namespace>.svc.cluster.local`.
- **Armazenamento estável por réplica**, via `volumeClaimTemplates`: cada
  Pod ganha sua própria `PersistentVolumeClaim`, criada automaticamente,
  que sobrevive a reinícios do Pod.
- **Ordem de criação/remoção** (dependendo do `podManagementPolicy`, ver
  abaixo).

## `serviceName` e o Service headless

```yaml
spec:
  serviceName: mysql   # precisa apontar para um Service headless existente
```

O `serviceName` só funciona se existir um Service do tipo **headless**
(`clusterIP: None`) com esse nome, selecionando os mesmos Pods:

```yaml
# k8s/mysql-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: mysql
spec:
  clusterIP: None        # "headless" — não cria um IP único de load balancer,
                          # só registra o DNS de cada Pod individualmente
  selector:
    app: mysql
  ports:
    - name: mysql
      port: 3306
```

Com isso, cada Pod fica alcançável pelo nome — ex.: `mysql-0.mysql` resolve
para o IP exato do Pod `mysql-0`. Isso é o que permite, por exemplo, uma
réplica de banco apontar explicitamente para outra (`mysql-0.mysql` como
"primary") em uma configuração de replicação — um Deployment não consegue
oferecer isso, porque os Pods não têm nomes fixos.

⚠️ **O Kubernetes não valida se o Service referenciado em `serviceName`
existe.** Se você errar o nome (ou esquecer de criar o Service), o
StatefulSet e os Pods são criados normalmente — só a resolução de DNS
(`<pod>.<serviceName>`) é que não vai funcionar. É um erro silencioso:
nada aparece em `kubectl get pods`, só falha na hora de um Pod tentar
resolver o nome de outro.

## `podManagementPolicy`: `OrderedReady` (padrão) vs `Parallel`

- **`OrderedReady`** (padrão, se omitido): cria/atualiza/remove os Pods
  **um de cada vez**, em ordem (criação: `0 → 1 → 2`; remoção: ordem
  inversa), esperando cada Pod ficar `Running` e `Ready` antes de mexer no
  próximo. É mais seguro para bancos que dependem de inicialização em
  ordem (ex.: um "primary" precisa existir antes das réplicas se
  conectarem a ele).
  - **Risco real que já vivemos neste projeto**: se um Pod nunca fica
    `Ready` (ex.: crash loop por falta de `MYSQL_ROOT_PASSWORD`), o
    rollout **trava inteiro** — nenhum outro Pod é criado/atualizado até
    aquele ficar `Ready`, o que pode nunca acontecer. Foi exatamente o
    que aconteceu quando adicionamos a senha ao YAML e reaplicamos: os
    Pods antigos (sem senha) nunca ficavam prontos, então o StatefulSet
    nunca recriava nenhum deles com o template novo — foi preciso apagar
    e recriar o objeto manualmente.
- **`Parallel`**: cria/remove **todos os Pods ao mesmo tempo**, sem
  esperar prontidão entre eles. Evita o travamento acima, mas abre mão da
  garantia de ordem — não use se a aplicação realmente depende de um Pod
  existir antes do outro.

O `k8s/statefulset.yaml` atual deste projeto já está configurado com
`podManagementPolicy: Parallel` — uma mudança deliberada, provavelmente
feita depois do travamento que tivemos com `OrderedReady`.

## `volumeClaimTemplates`: uma PVC por réplica, automaticamente

```yaml
spec:
  volumeClaimTemplates:
    - metadata:
        name: mysql-data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 1Gi
```

Ao contrário de um volume comum (declarado uma vez em
`spec.template.spec.volumes`, como no `goserver-volume` de
[k8s/deployment.yaml](../k8s/deployment.yaml)/[k8s/pvc.yaml](../k8s/pvc.yaml),
que é **uma PVC compartilhada entre todas as réplicas**), o StatefulSet
cria **uma PVC nova por Pod**, nomeada
`<nome-do-template>-<nome-do-pod>`:

```
mysql-data-mysql-0
mysql-data-mysql-1
mysql-data-mysql-2
```

Cada PVC vira um `PersistentVolume` real (aqui, provisionado
automaticamente pelo `local-path-provisioner` do `kind`, a StorageClass
`standard` — confira com `kubectl get storageclass`). Isso é o que garante
que os dados do `mysql-1`, por exemplo, continuem os mesmos mesmo que o
Pod seja destruído e recriado — o Pod novo `mysql-1` reconecta na mesma
PVC `mysql-data-mysql-1`.

### ⚠️ PVCs sobrevivem à exclusão do StatefulSet (comportamento real, observado agora)

Por padrão, **apagar um StatefulSet não apaga as PVCs criadas por ele** —
elas ficam órfãs, mas continuam `Bound`, ocupando armazenamento. Isso
aconteceu neste projeto agora mesmo: o `k8s/statefulset.yaml` foi
alterado (removendo temporariamente `volumeMounts`/`volumeClaimTemplates`,
comentados no arquivo) e o objeto foi recriado — as PVCs antigas
(`mysql-data-mysql-0`, `mysql-data-mysql-1`, `mysql-data-mysql-2`)
continuam existindo e `Bound`, só que **nenhum dos 5 Pods atuais monta
elas** (o StatefulSet vivo agora não tem `volumeClaimTemplates`
nenhum). Confira com:

```bash
kubectl get pvc
```

Esse comportamento é proposital: existe pra evitar perda acidental de
dados (se você recriar o StatefulSet, ele pode voltar a reconectar nas
PVCs antigas, contanto que os nomes batam). Mas também significa que
**apagar um StatefulSet não é suficiente para limpar o storage** — é
preciso `kubectl delete pvc` manualmente, ou configurar
`spec.persistentVolumeClaimRetentionPolicy` (Kubernetes 1.27+) para
automatizar essa limpeza em cenários específicos (`whenDeleted`/
`whenScaled`).

## Escalando um StatefulSet

```bash
kubectl scale statefulset mysql --replicas=5
```

Ajusta o número de réplicas **imperativamente** (direto no cluster, sem
passar pelo arquivo YAML) — equivalente ao que `kubectl scale deployment`
faz para um Deployment. Cada novo Pod (`mysql-3`, `mysql-4`, ...) recebe
seu próprio ordinal e, se `volumeClaimTemplates` estiver configurado,
sua própria PVC nova; ao escalar para baixo, os Pods de maior ordinal são
removidos primeiro, mas **as PVCs deles não são apagadas** (mesmo alerta
da seção acima).

### ⚠️ Drift entre `kubectl scale` (imperativo) e o YAML (declarativo)

Esse é o mesmo tipo de armadilha já documentada para o HPA em
[docs/autoscaling.md](autoscaling.md#️-cuidado-replicas-fixo-no-deployment-vs-hpa):
se o campo `replicas` **voltar** a aparecer no `k8s/statefulset.yaml` (hoje
está comentado, `# replicas: 8`) e alguém rodar `kubectl apply -f
k8s/statefulset.yaml` de novo, o valor do arquivo **substitui** o valor
definido por `kubectl scale` — a réplica 5 criada agora seria perdida na
próxima aplicação do manifest, se o arquivo disser outro número. Regra
prática: se o número de réplicas é ajustado manualmente com frequência
(ou por um HPA), é mais seguro deixar o campo `replicas` de fora do
manifest versionado, como está agora — assim `kubectl apply` nunca briga
com o valor real do cluster.

## Estado atual deste projeto (StatefulSet `mysql`)

Snapshot real, no momento em que este documento foi escrito — útil pra
comparar com o que o arquivo `k8s/statefulset.yaml` versionado no Git
realmente reflete (podem estar dessincronizados, já que o cluster foi
ajustado por comandos imperativos e edições locais ainda não commitadas):

| Campo | Valor ao vivo no cluster |
|---|---|
| `replicas` | `5` (via `kubectl scale`, não está no YAML) |
| `serviceName` | `mysql-h` — **não existe nenhum Service com esse nome** (o Service headless real se chama `mysql`); a resolução de DNS `<pod>.mysql-h` não funciona |
| `podManagementPolicy` | `Parallel` |
| `volumeClaimTemplates` | nenhum — os 5 Pods atuais **não montam nenhum volume persistente** |
| PVCs órfãs | `mysql-data-mysql-0`, `mysql-data-mysql-1`, `mysql-data-mysql-2` — `Bound`, mas sem nenhum Pod usando elas agora |

Isso não foi corrigido de propósito (fica registrado aqui como parte do
material de estudo da sessão de volumes) — se a intenção for religar a
persistência, os passos seriam: criar/renomear o Service headless para
bater com `serviceName` (`mysql-h`), descomentar `volumeMounts` no
container e `volumeClaimTemplates` no StatefulSet, e (por serem campos
imutáveis) apagar e recriar o objeto — mesmo procedimento já feito uma vez
neste projeto.

## Comandos úteis

```bash
kubectl get statefulset mysql                      # nome, réplicas prontas, idade
kubectl describe statefulset mysql                  # spec completa + eventos
kubectl rollout status statefulset mysql             # acompanha um rollout (útil pra pegar travamentos do OrderedReady)

kubectl scale statefulset mysql --replicas=5          # escala imperativamente (comando desta sessão)

kubectl get pods -l app=mysql -o wide                 # nomes ordinais, IPs, status
kubectl get pvc                                        # uma PVC por réplica (mysql-data-mysql-N) + as PVCs "soltas" (goserver-pvc)

kubectl exec -it mysql-0 -- mysql -uroot -proot        # entra no MySQL do Pod mysql-0
kubectl run -it --rm dns-test --image=busybox --restart=Never -- nslookup mysql-0.mysql
                                                         # confirma a identidade de rede (DNS) de um Pod específico

kubectl delete statefulset mysql                        # apaga o StatefulSet — NÃO apaga as PVCs
kubectl delete pvc -l app=mysql                          # limpa as PVCs órfãs manualmente, se quiser começar do zero
```
