# Service

Pods são **efêmeros**: eles podem morrer e ser recriados a qualquer
momento, e cada vez que isso acontece o Pod recebe um **IP novo**. Isso
torna inviável acessar Pods diretamente pelo IP.

Um **Service** resolve esse problema: é um objeto que dá um nome estável e
um IP/DNS fixo para um grupo de Pods, selecionados por `labels` (não
importa quantos Pods existem nem quais IPs eles têm no momento — o Service
sempre encontra os Pods certos e distribui o tráfego entre eles, como um
load balancer interno).

## Tipos de Service

- **ClusterIP** (padrão): expõe o Service apenas dentro do cluster. Usado
  para comunicação entre aplicações internas.
- **NodePort**: além do ClusterIP, abre uma porta fixa (faixa
  `30000–32767`) em **todos os nodes** do cluster, permitindo acesso
  externo via `<IP-do-node>:<nodePort>`.
- **LoadBalancer**: pede ao provedor de nuvem (AWS, GCP, Azure, …) para
  provisionar um load balancer externo real, com IP público, que
  encaminha tráfego para o Service. É uma extensão do NodePort.

## O Service deste projeto (`k8s/service.yaml`)

```yaml
apiVersion: v1
kind: Service
metadata:
  name: goserver-service
spec:
  selector:
    app: goserver
  type: LoadBalancer
  ports:
  - name: goserver-service
    port: 80
    targetPort: 8000
    protocol: TCP
    # nodePort: 30001 Para que serve?
```

Campo a campo:

- `spec.selector.app: goserver`: seleciona todos os Pods com o label
  `app: goserver` — exatamente os que o [Deployment](deployments.md) cria.
- `spec.type: LoadBalancer`: pede um load balancer externo.
- `port: 80`: a porta em que o **Service** escuta (a porta que quem
  consome o serviço usa).
- `targetPort: 8000`: a porta do **container** para onde o tráfego é
  encaminhado.

### O que é o `nodePort` comentado?

`nodePort` só faz sentido em Services do tipo `NodePort` (ou
`LoadBalancer`, que internamente também usa um NodePort). Ele fixa
manualmente qual porta (entre `30000` e `32767`) será aberta em cada node
para acesso externo. Se você **não** definir `nodePort`, o Kubernetes
escolhe uma porta livre aleatoriamente nessa faixa. Definir manualmente só
é útil quando você precisa de uma porta previsível (ex.: liberar
exatamente a porta `30001` no firewall). Para este projeto, não é
necessário — pode deixar comentado/omitido.

### Observação sobre a porta

O processo Go escuta em `:80` (veja [server.go](../server.go)), mas
`targetPort: 8000` aponta para a porta `8000`, que **nenhum processo do
container está escutando**. Para o Service realmente conseguir falar com a
aplicação, `targetPort` deveria ser `80` (a porta real do `server.go`), não
`8000`. Esse tipo de mismatch é a causa mais comum de "apliquei o Service
mas não consigo acessar a aplicação" no dia a dia com Kubernetes — sempre
confira: porta do processo → `targetPort` do Service → `port` do Service.

## Acessando o Service no kind

Clusters criados com `kind` **não** provisionam um load balancer real de
nuvem: um Service `type: LoadBalancer` fica com `EXTERNAL-IP` em
`<pending>` para sempre, a menos que você instale um add-on como o
`cloud-provider-kind`. Para acessar a aplicação localmente, use uma destas
opções:

```bash
# Opção 1: port-forward (mais simples para desenvolvimento)
kubectl port-forward svc/goserver-service 8080:80
curl localhost:8080

# Opção 2: ver o status do Service (EXTERNAL-IP ficará <pending>)
kubectl get svc goserver-service
```

## Comandos úteis

```bash
kubectl apply -f k8s/service.yaml     # cria/atualiza o Service
kubectl get svc                        # lista Services
kubectl describe svc goserver-service  # detalhes, endpoints associados
kubectl get endpoints goserver-service  # confere quais Pods foram selecionados
kubectl delete -f k8s/service.yaml     # remove o Service
```
