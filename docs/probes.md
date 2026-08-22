# Probes (liveness e readiness)

Uma **probe** é uma verificação de saúde que o **kubelet** faz periodicamente
contra um container, para decidir automaticamente o que fazer com o Pod.
Sem probes, o Kubernetes só sabe se o processo dentro do container está de
pé (PID rodando) — não sabe se a aplicação está de fato respondendo ou
pronta para receber tráfego.

Existem três tipos de probe, e este projeto usa as três no
`deployment.yaml`.

- **livenessProbe** — "o processo está vivo?"
- **readinessProbe** — "o processo está pronto para receber tráfego?"
- **startupProbe** — "o processo já terminou de inicializar?"

## livenessProbe: reinicia o container se ele travar

Se a `livenessProbe` falhar repetidamente, o kubelet entende que o container
está travado/quebrado e **mata e recria o container** (self-healing).

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 80
  periodSeconds: 5
  failureThreshold: 1
  timeoutSeconds: 1
  successThreshold: 1
  initialDelaySeconds: 15
```

- `httpGet.path` / `httpGet.port`: o kubelet faz `GET http://<pod-ip>:80/healthz`
  a partir do node — por isso a porta aqui precisa ser a porta real em que o
  processo escuta (`:80` no `server.go`), e não a porta "documentativa" do
  `containerPort`. Veja a observação sobre porta em
  [deployments.md](deployments.md#observação-sobre-a-porta).
- `initialDelaySeconds: 15`: espera 15s após o container iniciar antes da
  primeira checagem.
- `periodSeconds: 5`: depois disso, verifica a cada 5 segundos.
- `timeoutSeconds: 1`: espera no máximo 1s pela resposta antes de considerar
  falha.
- `failureThreshold: 1`: **uma única falha já é suficiente** para o kubelet
  considerar o container "não vivo" e reiniciá-lo (mais agressivo que o
  padrão, que é 3).
- `successThreshold: 1`: **sempre precisa ser `1`** em `livenessProbe` (e
  `startupProbe`) — a API do Kubernetes rejeita qualquer outro valor aqui.
  Só `readinessProbe` aceita `successThreshold` diferente de 1.

⚠️ Se a porta ou o path estiverem errados, a probe falha para sempre e o
Pod entra em **CrashLoopBackOff** — o container é reiniciado em loop porque
a cada boot ele parece "vivo" por um instante e logo a probe volta a falhar.

## readinessProbe: tira o Pod da rota sem matar o container

Se a `readinessProbe` falhar, o kubelet **não mata o container** — ele só
marca o Pod como "não pronto" (`READY 0/1`) e o Service para de mandar
tráfego para ele, até a probe voltar a passar.

```yaml
readinessProbe:
  httpGet:
    path: /healthz
    port: 80
  periodSeconds: 3
  failureThreshold: 1
  initialDelaySeconds: 10
```

Mesmos campos da `livenessProbe`, com destaque para:

- `initialDelaySeconds: 10`: espera 10s após o container iniciar antes da
  primeira checagem — dá tempo da aplicação subir antes de começar a
  receber tráfego (ou ser considerada "não pronta" logo de cara).
- `periodSeconds: 3` e `failureThreshold: 1`: checa a cada 3s e uma única
  falha já tira o Pod da rota — mais rápido e mais sensível que a
  `livenessProbe` (5s / falha única, mas com 15s de delay inicial). Isso é
  proposital: a readiness reage primeiro.
- `timeoutSeconds` e `successThreshold` não foram declarados, então usam o
  padrão do Kubernetes: `1` para ambos.

## startupProbe: protege a inicialização antes das outras duas entrarem em ação

Enquanto a `startupProbe` **não passar pela primeira vez**, o kubelet
**desliga** completamente a `livenessProbe` e a `readinessProbe` — elas nem
começam a rodar. Só depois que a `startupProbe` tem sucesso uma vez é que o
kubelet passa a chamar liveness/readiness normalmente, e a `startupProbe`
para de ser executada.

```yaml
startupProbe:
  httpGet:
    path: /healthz
    port: 80
  periodSeconds: 3
  failureThreshold: 30
```

(bloco real do [k8s/deployment.yaml](../k8s/deployment.yaml) — janela de até
`30 × 3s = 90s` para o processo inicializar)

- Objetivo: dar uma **janela de tolerância só para o boot**, sem precisar
  inflar o `initialDelaySeconds`/`failureThreshold` da `livenessProbe` para
  cobrir o pior caso de inicialização.
- `failureThreshold * periodSeconds` define o tempo máximo tolerado de boot
  (no exemplo, até 150s). Se a app não responder `200` nesse prazo, o
  container é morto e reiniciado — mesmo comportamento de falha da
  `livenessProbe`.
- Não existe `readinessProbe`/`livenessProbe` "correndo em paralelo" durante
  esse período: é uma fase exclusiva, sequencial.

### Ela é realmente necessária aqui?

Na prática, não muito: o binário Go sobe em milissegundos e a rota
`/healthz` já responde de imediato (mesmo que "falsamente" `500` nos
primeiros 10s, por causa da lógica descrita abaixo). Não existe boot lento
e variável para proteger — o `initialDelaySeconds: 15` da `livenessProbe`
sozinho já cobriria essa janela fixa e curta. Ela está presente aqui com
fins didáticos, para o comportamento poder ser comparado na prática com os
outros dois tipos de probe.

## Como isso se conecta com a rota `/healthz` deste projeto

O handler `Healthz` em [server.go](../server.go#L40-L49) simula
deliberadamente uma janela de **boot lento**, baseada no tempo de vida
(`uptime`) do processo:

```go
func Healthz(w http.ResponseWriter, r *http.Request) {
	duration := time.Since(startedAt)

	if duration.Seconds() < 10 {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("Duration: %v", duration.Seconds())))
	} else {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}
}
```

Ou seja: `500` enquanto o container tem **menos de 10s** de vida, e `200`
**para sempre** depois disso. Cruzando isso com os tempos das três probes:

| Tempo (uptime) | `/healthz` | O que acontece |
|---|---|---|
| `0s` – `10s` | `500` | `startupProbe` ainda checando (a cada 3s); `readinessProbe`/`livenessProbe` ficam pausadas até ela passar |
| `10s` (1ª checagem que bate `duration >= 10`) | `200` | `startupProbe` passa → kubelet libera `readinessProbe`/`livenessProbe` para começar |
| a partir daqui | `200` sempre | Pod fica `READY 1/1` e nunca mais falha nenhuma probe por conta própria |

Repare que esse handler **já existiu com outro comportamento**: uma versão
anterior (preservada em [server.go](../server.go#L52-L63) como
`HealthzOld`, comentada) também voltava a devolver `500` depois de `30s` de
vida, criando um ciclo infinito de restart. Isso foi proposital — servia
para observar o ciclo completo `readinessProbe` → `livenessProbe` → restart
descrito nas seções acima. Só que esse ciclo **atrapalha** o teste de HPA
(ver [autoscaling.md](autoscaling.md)): um Pod sendo morto e recriado a
cada ~35s durante um teste de carga sustentado confunde a leitura de CPU
do HPA. Por isso o handler atual só simula um **boot lento de 10s** e
depois fica estável — ideal para observar `startupProbe`/`readinessProbe`
na inicialização, sem interferir num teste de autoscaling que dura minutos.

## Preciso das três no Deployment, ou uma já basta?

Nenhuma probe é obrigatória — sem nenhuma, o Kubernetes só sabe se o PID
está de pé, e é isso. A escolha depende do que a aplicação precisa:

| Cenário | O que usar | Por quê |
|---|---|---|
| App simples, boot rápido e previsível (ex.: este projeto) | `readinessProbe` + `livenessProbe` | Não há fase de boot lenta o suficiente para justificar uma `startupProbe`; um `initialDelaySeconds` fixo na liveness já cobre o boot. |
| Só quer parar de mandar tráfego pra Pod não pronto, sem restart automático | Só `readinessProbe` | Aceita ficar com Pods "travados" (não-prontos) para sempre em vez de reiniciá-los — geralmente arriscado. |
| Só quer self-healing, sem se importar em mandar tráfego pra Pod ainda subindo | Só `livenessProbe` | Risco de erros para quem chama, se a app não estiver pronta mas já responder à liveness. Combinação pouco comum. |
| Boot lento e/ou de duração variável (JVM aquecendo, migração de banco, carregar modelo/cache grande) | As três: `startupProbe` + `readinessProbe` + `livenessProbe` | `startupProbe` absorve o tempo de boot sem arriscar matar o container cedo demais nem obrigar a liveness a ter um `initialDelaySeconds` gigante (o que a deixaria lenta pra detectar travamentos reais depois). |

Resumindo os papéis para não confundir:

- **`startupProbe`** — cuida só do **momento do boot**. Enquanto não passa,
  as outras duas ficam pausadas.
- **`readinessProbe`** — cuida do **tráfego**, continuamente, durante toda a
  vida do Pod: entra/sai da rota do Service.
- **`livenessProbe`** — cuida da **sobrevivência do processo**,
  continuamente, durante toda a vida do Pod: mata e recria se travar.

As duas juntas (`readinessProbe` + `livenessProbe`), como usado neste
projeto: o Service só recebe tráfego de Pods realmente prontos, e Pods que
travam de vez são recriados automaticamente. A `startupProbe` entra na
jogada quando o boot em si já é o problema a ser protegido.

## Comandos úteis para depurar probes

```bash
kubectl get pods                          # READY 0/1 = falhando readiness; CrashLoopBackOff = falhando liveness
kubectl describe pod <nome-do-pod>        # seção Events mostra "Liveness probe failed" / "Readiness probe failed"
kubectl logs <nome-do-pod>                # logs da aplicação no momento da falha
kubectl logs <nome-do-pod> --previous     # logs da execução anterior (útil após um restart)
```
