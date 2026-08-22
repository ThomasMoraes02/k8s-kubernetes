# Deployment

Um **Deployment** é o objeto do Kubernetes usado para declarar como uma
aplicação deve rodar: qual imagem usar, quantas réplicas (cópias) do Pod
manter no ar, e como atualizar essas réplicas sem downtime.

Por baixo dos panos, o Deployment cria e gerencia um **ReplicaSet**, que é
quem efetivamente garante que o número de Pods "vivos" seja sempre igual ao
declarado (`replicas`). Se um Pod morrer, o ReplicaSet cria outro
automaticamente.

Diferente de um Pod criado diretamente (veja [pods.md](pods.md)), o
Deployment é a forma recomendada de rodar aplicações em produção, porque
oferece:

- **Réplicas**: várias cópias do mesmo Pod, para alta disponibilidade e
  distribuição de carga.
- **Rolling updates**: ao trocar a imagem, o Kubernetes substitui os Pods
  antigos pelos novos aos poucos, sem tirar a aplicação do ar.
- **Rollback**: é possível voltar para a revisão anterior caso um deploy dê
  problema.
- **Self-healing**: Pods que falham são recriados automaticamente.

## O Deployment deste projeto (`k8s/deployment.yaml`)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: goserver
  labels:
    app: goserver
spec:
  selector:
    matchLabels:
      app: goserver
  replicas: 10
  template:
    metadata:
      labels:
        app: "goserver"
    spec:
      containers:
      - name: goserver
        image: "thomasmoraes02/hello-go:latest"
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
          limits:
            cpu: "200m"
            memory: "256Mi"
        ports:
        - containerPort: 8080
```

Campo a campo:

- `spec.replicas: 10`: mantém 10 Pods idênticos rodando o tempo todo.
- `spec.selector.matchLabels` / `template.metadata.labels`: precisam ser
  iguais (`app: goserver`) — é assim que o Deployment sabe quais Pods são
  "dele".
- `spec.template`: é literalmente a definição de um Pod (mesma estrutura de
  [k8s/pod.yaml](../k8s/pod.yaml)), usada como molde para cada réplica.
- `resources.requests`: quanto de CPU/memória o Kubernetes **reserva** para
  o container ao decidir em qual node colocá-lo.
- `resources.limits`: o **teto** de CPU/memória que o container pode
  consumir; ultrapassar o limite de memória causa `OOMKilled`.
- `ports.containerPort`: apenas **documentação** de qual porta o container
  expõe — não abre porta nenhuma sozinho (quem faz isso é o Service).

### Observação sobre a porta

`containerPort: 8080` é meramente informativo, então o Deployment funciona
independente do valor. Mas o processo Go (`server.go`) na verdade escuta em
`:80` (`http.ListenAndServe(":80", nil)`), então o valor correto aqui seria
`containerPort: 80` para a documentação bater com a realidade. Quem
realmente precisa apontar para a porta certa é o `targetPort` do Service —
veja [services.md](services.md#observação-sobre-a-porta).

## Comandos úteis

```bash
kubectl apply -f k8s/deployment.yaml        # cria/atualiza o Deployment
kubectl get deployments                      # lista Deployments
kubectl get pods -l app=goserver              # lista os Pods do Deployment
kubectl describe deployment goserver          # detalhes e eventos
kubectl scale deployment goserver --replicas=3  # muda o nº de réplicas
kubectl rollout status deployment goserver     # acompanha um rollout
kubectl rollout history deployment goserver    # histórico de revisões
kubectl rollout undo deployment goserver        # rollback para a anterior
kubectl delete -f k8s/deployment.yaml          # remove o Deployment

kubectl apply -f k8s/deployment.yaml && watch -n1 kubectl get pods
```
