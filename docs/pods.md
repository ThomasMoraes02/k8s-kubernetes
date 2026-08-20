# Pod

Um **Pod** é a menor unidade que pode ser criada e gerenciada no
Kubernetes: dentro dele roda um ou mais containers, que compartilham a
mesma rede (mesmo IP e portas) e podem compartilhar volumes de
armazenamento. Na prática, é como se fosse "uma máquina" com container(es),
porta(s) e IP próprios dentro do cluster.

Na grande maioria dos casos um Pod contém **um único container** (é o caso
deste projeto); múltiplos containers no mesmo Pod são usados em padrões
específicos, como o de "sidecar" (ex.: um proxy ou coletor de logs rodando
ao lado da aplicação principal).

Pods são **efêmeros**: se um Pod morre, ele não "renasce" sozinho — ele
some. É por isso que, na prática, quase nunca se cria um Pod diretamente
como no exemplo abaixo; usa-se um [Deployment](deployments.md), que recria
Pods automaticamente quando eles falham.

## O Pod deste projeto (`k8s/pod.yaml`)

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: "goserver"
  labels:
    app: "goserver"
spec:
  containers:
    - name: "goserver"
      image: "thomasmoraes02/hello-go:latest"
```

Campo a campo:

- `apiVersion` / `kind`: dizem ao Kubernetes que tipo de recurso é este
  (aqui, `v1/Pod`).
- `metadata.name`: nome do Pod dentro do cluster.
- `metadata.labels`: rótulos usados por outros objetos (como Services e
  Deployments) para "encontrar" este Pod.
- `spec.containers`: lista de containers dentro do Pod — aqui, um único
  container chamado `goserver`, rodando a imagem
  `thomasmoraes02/hello-go:latest`.

## Comandos úteis

```bash
kubectl apply -f k8s/pod.yaml       # cria o Pod
kubectl get pods                     # lista os Pods
kubectl describe pod goserver        # detalhes e eventos do Pod
kubectl logs goserver                # logs do container
kubectl exec -it goserver -- sh      # abre um shell dentro do container
kubectl delete -f k8s/pod.yaml       # remove o Pod
```

> Para rodar isso com múltiplas réplicas e recriação automática em caso de
> falha, veja [deployments.md](deployments.md).
