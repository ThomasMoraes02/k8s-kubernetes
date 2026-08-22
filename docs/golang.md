# Build e execução da aplicação Go

A aplicação (`server.go`) é um servidor HTTP simples que responde
`"Hello Full Cycle!"` na porta `80`. Ela é empacotada em uma imagem Docker
pelo `Dockerfile`:

```dockerfile
FROM golang:1.15-alpine

COPY . .

RUN go build -o server .

CMD ["./server"]
```

## Comandos úteis

```bash
# builda a imagem Docker a partir do Dockerfile, com a tag thomasmoraes02/hello-go
docker build -t thomasmoraes02/hello-go .

# roda a imagem localmente, mapeando a porta 80 do container para a 80 do host
docker run --rm -p 80:80 thomasmoraes02/hello-go

# testa localmente
curl localhost
```

Isso é útil para validar que a imagem funciona **antes** de levá-la para o
Kubernetes. O próximo passo é subir um cluster local e disponibilizar essa
mesma imagem para ele — veja
[kind.md](kind.md#carregando-uma-imagem-local-no-kind).

## Por que toda alteração no `server.go` exige um rebuild

O Kubernetes **nunca lê o seu `server.go` do disco**. O que roda dentro do
Pod é o binário `server` que foi compilado **dentro da imagem Docker**, no
passo `RUN go build -o server .` do `Dockerfile`, no momento do `docker
build`. Uma vez construída, uma imagem Docker é **imutável** — ela é um
conjunto de camadas (layers) congeladas; editar `server.go` no seu editor
não muda um único byte de uma imagem já buildada.

Ou seja: o loop "editei o código → preciso ver isso rodando no cluster"
sempre passa pelas mesmas três etapas, porque cada uma resolve um problema
diferente:

1. **Buildar de novo** — gera uma imagem nova, com o binário novo lá dentro.
2. **Publicar (`push`) essa imagem** — o cluster Kubernetes (mesmo local,
   como o `kind`) não enxerga o Docker do seu host; ele só consegue puxar
   imagens de um registry, então a imagem nova precisa estar acessível de
   algum lugar que o cluster alcance.
3. **Apontar o Deployment para a tag nova e reaplicar** — o manifest
   (`k8s/deployment.yaml`) referencia uma tag específica da imagem; até você
   trocar essa tag e reaplicar, o Kubernetes não tem motivo para saber que
   existe uma versão nova.

### Comando a comando

```bash
docker build -t thomasmoraes02/hello-go:v5.6 .
```

- Reexecuta o `Dockerfile` (`COPY . .` + `go build`), empacotando o
  `server.go` atual em uma imagem nova.
- `-t thomasmoraes02/hello-go:v5.6`: dá um nome e uma **tag** à imagem
  gerada. Formato é sempre `<usuário-ou-org>/<repositório>:<tag>` — aqui,
  `thomasmoraes02` é o usuário no Docker Hub, `hello-go` o nome do
  repositório, `v5.6` a versão desta build específica.
- `.` no final: o **build context** — a pasta cujo conteúdo fica disponível
  para instruções como `COPY . .` dentro do `Dockerfile`; aqui é o diretório
  atual (`kubernetes/`).
- Essa imagem, nesse momento, só existe no Docker **local** da sua máquina.

```bash
docker push thomasmoraes02/hello-go:v5.6
```

- Envia a imagem `v5.6` para o **registry** configurado (por padrão, Docker
  Hub — `docker.io`), sob a conta `thomasmoraes02`. Exige estar autenticado
  (`docker login`) e ter permissão de escrita nesse repositório.
- É só depois desse `push` que qualquer cluster Kubernetes (o `kind` local,
  ou um cluster real em produção) consegue fazer `docker pull` dessa
  imagem — veja a Opção 2 em
  [kind.md](kind.md#opção-2--publicar-a-imagem-em-um-registry-docker-hub).

```bash
kubectl apply -f k8s/deployment.yaml
```

- Só funciona se, antes disso, o campo `image:` do
  [k8s/deployment.yaml](../k8s/deployment.yaml) tiver sido editado para
  `thomasmoraes02/hello-go:v5.6` (o `push` sozinho não atualiza o cluster —
  o Deployment continua apontando para a tag antiga até você editar o
  YAML).
- O Kubernetes compara o novo manifest com o estado atual do Deployment.
  Como o campo `image` mudou, ele entende que o Pod template mudou e
  dispara um **rolling update**: cria Pods novos com `v5.6`, espera cada um
  passar pela `readinessProbe` ([probes.md](probes.md)), e só então desliga
  os Pods antigos com `v5.5` — sem downtime.

## Por que usar tags de versão (`v5.6`) em vez de `latest`

- **O rollout só dispara porque a tag mudou.** O Kubernetes decide se
  precisa recriar Pods comparando o *texto* do campo `image` no manifest.
  Se você sempre usasse a mesma tag (`latest`) e só trocasse o conteúdo da
  imagem por trás dela, `kubectl apply` veria o mesmo texto `image:
  thomasmoraes02/hello-go:latest` de antes, concluiria que **nada mudou** e
  não recriaria nenhum Pod — mesmo com uma imagem nova já publicada no
  registry. Precisaria forçar manualmente com `kubectl rollout restart
  deployment goserver`.
- **Rollback deixa de ser possível de verdade.** `kubectl rollout undo`
  ([deployments.md](deployments.md)) funciona porque cada revisão do
  Deployment guarda qual tag de imagem estava em uso. Com `latest`, todas
  as revisões apontam para o mesmo texto `latest` — não há como o
  Kubernetes saber qual *conteúdo* de imagem cada revisão realmente usava.
  Com `v5.5`, `v5.6`, etc., cada revisão é rastreável e reversível de
  verdade.
- **`imagePullPolicy` muda de comportamento.** Como visto em
  [kind.md](kind.md#opção-1--carregar-a-imagem-diretamente-no-cluster-kind-recomendado-para-dev),
  a tag `latest` força `imagePullPolicy: Always` (sempre tenta rebaixar do
  registry). Uma tag versionada como `v5.6` usa o padrão
  `imagePullPolicy: IfNotPresent` — o kubelet reaproveita a imagem se já a
  tiver localmente, evitando pulls desnecessários.
- **Rastreabilidade**: com `kubectl describe pod` ou `kubectl get deployment
  -o yaml`, dá pra saber exatamente qual versão do código está rodando em
  produção. Com `latest`, essa pergunta não tem resposta confiável.
