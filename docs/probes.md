# Probes (liveness e readiness)

Uma **probe** é uma verificação de saúde que o **kubelet** faz periodicamente
contra um container, para decidir automaticamente o que fazer com o Pod.
Sem probes, o Kubernetes só sabe se o processo dentro do container está de
pé (PID rodando) — não sabe se a aplicação está de fato respondendo ou
pronta para receber tráfego.

Existem três tipos de probe. Este projeto usa as duas primeiras no
`deployment.yaml`; a terceira é documentada abaixo por completude, com um
exemplo hipotético.

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
  periodSeconds: 5
  failureThreshold: 30   # 30 x 5s = até 150s para o processo inicializar
```

- Objetivo: dar uma **janela de tolerância só para o boot**, sem precisar
  inflar o `initialDelaySeconds`/`failureThreshold` da `livenessProbe` para
  cobrir o pior caso de inicialização.
- `failureThreshold * periodSeconds` define o tempo máximo tolerado de boot
  (no exemplo, até 150s). Se a app não responder `200` nesse prazo, o
  container é morto e reiniciado — mesmo comportamento de falha da
  `livenessProbe`.
- Não existe `readinessProbe`/`livenessProbe` "correndo em paralelo" durante
  esse período: é uma fase exclusiva, sequencial.

### Por que este projeto não tem uma

O binário Go deste projeto sobe em milissegundos e a rota `/healthz`
responde imediatamente (mesmo que "falsamente" `500` nos primeiros 10s, por
causa da lógica descrita abaixo). Não existe boot lento e variável para
proteger — o `initialDelaySeconds: 15` da `livenessProbe` já é suficiente
para cobrir essa janela fixa e curta. Uma `startupProbe` existe pra resolver
um problema que este projeto não tem.

## Como isso se conecta com a rota `/healthz` deste projeto

O handler `Healthz` em [server.go](../server.go#L40-L50) não verifica nada de
verdade — ele simula deliberadamente uma janela de saúde baseada no tempo
de vida (`uptime`) do processo:

```go
func Healthz(w http.ResponseWriter, r *http.Request) {
	duration := time.Since(startedAt)

	if duration.Seconds() < 10 || duration.Seconds() > 30 {
		w.WriteHeader(500)
		...
	} else {
		w.WriteHeader(http.StatusOK)
		...
	}
}
```

Ou seja: `500` enquanto o container tem **menos de 10s** de vida, `200`
entre **10s e 30s**, e `500` de novo depois de **30s**. Cruzando isso com os
tempos das duas probes acima, dá pra prever exatamente o ciclo de vida do
Pod:

| Tempo (uptime) | `/healthz` | O que acontece |
|---|---|---|
| `0s` – `10s` | `500` | Pod ainda não sofreu a 1ª checagem de readiness (`initialDelaySeconds: 10`); fica `READY 0/1` |
| `10s` (1ª readiness) | `200` | Readiness passa → Pod entra `READY 1/1`, Service passa a mandar tráfego |
| `15s` (1ª liveness) | `200` | Liveness passa → container segue vivo, sem reiniciar |
| `10s` – `30s` | `200` | Pod saudável e recebendo tráfego normalmente |
| `~31s` (readiness, período de 3s: 10,13,...,28,31) | `500` | 1ª falha já tira o Pod da rota (`failureThreshold: 1`) → volta a `READY 0/1`, mas o **container continua rodando** |
| `~35s` (liveness, período de 5s: 15,20,25,30,35) | `500` | 1ª falha já derruba o container (`failureThreshold: 1`) → kubelet mata e recria o container |
| depois do restart | — | `startedAt` reseta, o ciclo inteiro recomeça do zero |

O ponto didático aqui: como `periodSeconds` da readiness (3s) é menor que o
da liveness (5s) e o `initialDelaySeconds` da liveness é maior (15s vs 10s),
a **readiness sempre detecta o problema primeiro** (~31s) e só remove o Pod
da rota do Service — sem matar nada. A **liveness detecta um pouco depois**
(~35s) e aí sim reinicia o container. Rodando
`kubectl get pods -w` (ou `watch -n1 kubectl get pods`) dá pra ver esse
ciclo se repetindo: `0/1` → `1/1` → `0/1` (ready falha) → restart → `0/1` →
`1/1` → ...

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
