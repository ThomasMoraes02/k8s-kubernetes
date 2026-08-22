# Ingress

Um Service `LoadBalancer` (como o [`goserver-service`](../k8s/service.yaml))
dá um IP externo dedicado a **uma** aplicação. Isso funciona, mas não
escala bem: um cluster com 10 aplicações HTTP precisaria de 10 load
balancers externos, um por app — caro e desnecessário, já que todo tráfego
HTTP/HTTPS podia passar por um único ponto de entrada. É esse problema que
o **Ingress** resolve: ele expõe HTTP/HTTPS com **regras de roteamento por
host e/ou path**, na camada 7, direcionando tráfego para vários Services
internos `ClusterIP` a partir de um único ponto de entrada.

## Ingress vs Service

| Objeto | Camada | Cenário típico |
|---|---|---|
| `Service` (`ClusterIP`) | L4 (TCP) | Comunicação interna entre Pods |
| `Service` (`LoadBalancer`) | L4 (TCP) | Um IP público dedicado por serviço |
| `Ingress` | L7 (HTTP/HTTPS) | Um único ponto de entrada, roteando por host/path pra vários Services `ClusterIP` |

## O Ingress sozinho não faz nada — precisa de um Controller

Um objeto `Ingress` é só a **regra** ("host X, path Y → Service Z"). Quem
lê essa regra e efetivamente abre uma porta escutando tráfego HTTP é o
**Ingress Controller** — um componente à parte, instalado separadamente,
que não vem por padrão em nenhum cluster (nem `kind`, nem EKS/GKE/AKS
puros). Sem controller, aplicar um `Ingress` cria o objeto normalmente, mas
ele fica "morto" — nada escuta pra valer.

Controllers comuns: [ingress-nginx](https://kubernetes.github.io/ingress-nginx/)
(o mais usado, usado neste projeto), Traefik, HAProxy, ou específicos de
cloud (AWS Load Balancer Controller, GKE Ingress). Cada um lê objetos
`Ingress` marcados com sua `IngressClass` e configura a si mesmo — o objeto
`Ingress` em si é sempre o mesmo formato, portátil entre controllers.

## Instalando o ingress-nginx no kind

```bash
curl -sSL -o k8s/ingress-nginx.yaml \
  https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

kubectl apply -f k8s/ingress-nginx.yaml
```

Isso instala (no namespace `ingress-nginx`, criado pelo próprio manifest):
um `Deployment` com o controller de verdade (imagem `ingress-nginx`), RBAC
(`ServiceAccount`/`ClusterRole`/bindings) pra ele poder ler `Ingress`es do
cluster inteiro, um `Service` (`ingress-nginx-controller`), e uma
`IngressClass` chamada `nginx` — é essa `IngressClass` que qualquer
`Ingress` referencia pra dizer "eu quero que **este** controller me
atenda" (relevante se você tiver mais de um controller no mesmo cluster).

```bash
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=180s
```

### ⚠️ Por que não dá pra acessar direto em `localhost:80` neste projeto

O manifest oficial "kind" do ingress-nginx assume que o cluster foi criado
com **`extraPortMappings`** — uma configuração no `kind create cluster`
que mapeia as portas `80`/`443` do node (container Docker) para portas do
host Mac, algo como:

```yaml
# exemplo de kind-config.yaml (NÃO aplicado neste projeto)
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraPortMappings:
      - containerPort: 80
        hostPort: 80
      - containerPort: 443
        hostPort: 443
```

O cluster `fullcycle` deste projeto **foi criado sem isso**
(`kind create cluster --name fullcycle`, sem arquivo de config — confirmado
com `docker inspect fullcycle-control-plane`, que só mostra a porta `6443`
da API mapeada). Recriar o cluster com essa config apagaria tudo que já
está rodando nele (MySQL, goserver, HPA, etc.), então não foi feito aqui.

**Solução usada**: `port-forward` no Service do controller, exatamente
como já fazemos com o `goserver-service`:

```bash
kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 8081:80
```

## O Ingress deste projeto (`k8s/ingress.yaml`)

```yaml
apiVersion: networking.k8s.io/v1   # API do Ingress
kind: Ingress
metadata:
  name: goserver-ingress
spec:
  ingressClassName: nginx           # qual controller deve atender esta regra (o instalado acima)
  rules:
    - host: goserver.local           # só responde para requisições com este Host
      http:
        paths:
          - path: /                   # qualquer path a partir da raiz
            pathType: Prefix           # "Prefix" = bate com /, /qualquercoisa, /a/b/c...
            backend:
              service:
                name: goserver-service  # Service de destino (o mesmo já usado por port-forward)
                port:
                  number: 80             # porta do Service, não do container
```

- `pathType: Prefix` é o mais comum — bate com o path declarado e qualquer
  coisa abaixo dele. As alternativas são `Exact` (bate só o path exato) e
  `ImplementationSpecific` (regra livre, depende do controller).
- Como só existe uma aplicação neste projeto, essa regra é simples (um
  host, um path, um backend). O ganho real do Ingress aparece quando você
  tem várias aplicações — cada uma com seu próprio `host` ou `path`, todas
  atrás do mesmo controller/IP.

## Testando de ponta a ponta

Com o `port-forward` do controller ativo (seção acima), o roteamento é
feito pelo **header `Host`** — é assim que o nginx dentro do controller
decide qual `Ingress`/Service atender:

```bash
# Com o Host correto → cai no goserver
curl -H "Host: goserver.local" localhost:8081/
# Hello, I'm Thomas, Age: 26
# <h1>Hello Full Cycle!</h1>

curl -H "Host: goserver.local" localhost:8081/healthz
# ok

# Sem o Host (ou com um Host que não bate com nenhuma regra) → 404 do PRÓPRIO nginx,
# não do goserver — prova que o roteamento por host está realmente em vigor
curl localhost:8081/
# 404 Not Found (nginx)
```

Isso confirma duas coisas ao mesmo tempo: que o controller está no ar e
processando regras, e que a regra de host (`goserver.local`) está
funcionando de verdade — uma requisição sem esse `Host` **não** cai por
acidente no `goserver`.

### Testando com navegador (via `/etc/hosts`)

Pra acessar pelo navegador em vez de `curl -H`, mapeie o host localmente:

```bash
echo "127.0.0.1 goserver.local" | sudo tee -a /etc/hosts
kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 8081:80
```

E abra `http://goserver.local:8081/` no navegador. (A porta `:8081`
continua necessária aqui porque, como explicado acima, este cluster não
tem a porta `80` do host mapeada — só o hostname é resolvido via
`/etc/hosts`, a porta do `port-forward` continua entrando na URL.)

## TLS (HTTPS) — não configurado neste projeto

Um `Ingress` também pode terminar TLS, apontando pra um `Secret` do tipo
`kubernetes.io/tls` com o certificado:

```yaml
spec:
  tls:
    - hosts:
        - goserver.local
      secretName: goserver-tls
```

Na prática, certificados costumam ser emitidos automaticamente por uma
ferramenta como o [cert-manager](https://cert-manager.io/) (integrado ao
Let's Encrypt, por exemplo), que cria esse `Secret` sozinho. Fora do
escopo deste projeto (é um laboratório local, sem domínio público de
verdade), mas é o próximo passo natural em um cluster real como EKS.

## Comandos úteis

```bash
kubectl get ingress                              # lista Ingresses (mostra HOSTS, ADDRESS, PORTS)
kubectl describe ingress goserver-ingress          # regras + eventos de sincronização com o controller
kubectl get ingressclass                            # controllers disponíveis no cluster

kubectl get pods -n ingress-nginx                    # status do controller
kubectl logs -n ingress-nginx -l app.kubernetes.io/component=controller --tail=50
                                                       # logs do controller — útil pra depurar uma regra que não está roteando

kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 8081:80
                                                       # acesso local (necessário neste cluster kind, ver nota acima)

kubectl delete -f k8s/ingress.yaml                    # remove só a regra
kubectl delete -f k8s/ingress-nginx.yaml               # remove o controller inteiro
```
