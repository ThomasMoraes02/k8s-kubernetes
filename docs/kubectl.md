# kubectl

**kubectl** é a CLI usada para interagir com um cluster Kubernetes: ele
conversa com o `kube-apiserver` do cluster (configurado atualmente em
`~/.kube/config`) para criar, consultar, alterar e remover recursos
(Pods, Deployments, Services, etc.).

## Aplicar manifests

```bash
kubectl apply -f k8s/pod.yaml         # aplica um único arquivo
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/                 # aplica todos os manifests da pasta
```

`apply` é declarativo e idempotente: cria o recurso se não existir, ou
atualiza para bater com o que está no YAML se já existir.

## Consultar recursos

```bash
kubectl get pods                       # lista Pods
kubectl get pods -o wide               # com IP e node de cada Pod
kubectl get deployments                # lista Deployments
kubectl get svc                        # lista Services
kubectl get nodes                      # lista os nodes do cluster
kubectl get all                        # lista os recursos principais de uma vez

kubectl describe pod goserver           # detalhes e eventos de um recurso
kubectl describe deployment goserver
kubectl describe svc goserver-service
```

## Logs e depuração

```bash
kubectl logs goserver                  # logs de um Pod
kubectl logs goserver -f               # acompanha os logs em tempo real
kubectl logs -l app=goserver --tail=50 # logs de todos os Pods com esse label
kubectl exec -it goserver -- sh         # abre um shell dentro do container
```

## Acessando a aplicação sem um LoadBalancer real

```bash
kubectl port-forward svc/goserver-service 8080:80
kubectl port-forward pod/goserver 8080:80
```

## Alterando e removendo

```bash
kubectl scale deployment goserver --replicas=3
kubectl rollout restart deployment goserver
kubectl delete -f k8s/pod.yaml
kubectl delete deployment goserver
kubectl delete svc goserver-service
```

## Contexto (qual cluster estou usando?)

```bash
kubectl config get-contexts            # lista os clusters conhecidos
kubectl config use-context kind-fullcycle  # troca para o cluster do kind
kubectl cluster-info                    # informações do cluster atual
```
