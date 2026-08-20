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
