# Desenvolvimento local

[English](DEVELOPMENT.md) | Português

O ambiente local sobe o site Hugo com live reload e um loop de coleta que grava
snapshots novos, para você ver o painel se comportar como em produção sem
publicar nada.

## Requisitos

- Docker Engine 24+ com Compose v2
- `make`
- Go 1.27+ apenas para rodar a CLI fora do Docker

A imagem do Hugo está fixada no `docker-compose.yml`, então não é preciso
instalar Hugo na máquina.

## Início rápido

```bash
make dev/up      # compila e sobe Hugo + coletor
make dev/logs    # acompanha os dois serviços
```

Abra <http://localhost:1314>. O coletor valida o catálogo, grava
`site/static/data/state.json` e repete a cada 60 segundos. O Hugo serve o
snapshot em `/data/state.json` com `Cache-Control: no-store`.

Pare tudo com `make dev/down`.

## O que roda

| Serviço     | Imagem                       | Porta       | Papel                                                       |
| ----------- | ---------------------------- | ----------- | ----------------------------------------------------------- |
| `site`      | `hugomods/hugo:0.158.0`      | `1314→1313` | `hugo server`, live reload, monta `./site`.                 |
| `collector` | build do `Dockerfile.dev`    | nenhuma     | Loop de `seraph validate` + `seraph sync` a cada 60s.       |

A porta `1314` evita a `1313`, normalmente ocupada por outro projeto. Para
trocar:

```bash
make dev/up SITE_PORT=1414
make dev/logs
```

O intervalo também é configurável:

```bash
make dev/up DEV_INTERVAL=15
```

O Makefile passa seu `UID`/`GID` para os containers, então arquivos criados em
`site/static/data` e `.hugo_build.lock` continuam pertencendo ao seu usuário.

## Editando o site

| Caminho                         | Propósito                                      |
| ------------------------------- | ---------------------------------------------- |
| `site/hugo.toml`                | Config do Hugo, cache headers para `/data/**`. |
| `site/layouts/index.html`       | Markup do painel (template home do Hugo).      |
| `site/static/style.css`         | Estilos.                                       |
| `site/static/app.js`            | Fetch, filtros, favoritos, ligação do i18n.    |
| `site/static/i18n/en.js`        | Strings em inglês.                             |
| `site/static/i18n/pt-br.js`     | Strings em português do Brasil.                |
| `site/static/data/state.json`   | Snapshot gerado, ignorado pelo git.            |

Mudanças de HTML, CSS e JS recarregam o navegador automaticamente. Mantenha as
strings em inglês e português em sincronia ao adicionar texto de interface. Ao
alterar arquivos listados no shell do service worker, incremente `CACHE` em
`site/static/sw.js` para clientes instalados substituírem o shell antigo.

## Editando o coletor

O binário do coletor é compilado na imagem. Depois de mudanças em Go:

```bash
make dev/rebuild     # recompila a imagem e reinicia o serviço
```

Catálogo e configuração são montados como somente leitura, então adicionar um
serviço ou mudar `config/seraph.yml` vale no próximo ciclo, sem rebuild:

```bash
make dev/collect     # roda um ciclo imediatamente
```

## Builds parecidos com produção

```bash
make site-build      # build do Hugo em site/public
make preview         # compila, sobe em segundo plano e imprime a URL
make preview/logs    # acompanha requisições e erros
make preview/status  # confere se o servidor está rodando
make preview/stop    # para o servidor
```

`make preview` retorna imediatamente e grava em `.preview/server.log`. Use outra
porta com `PREVIEW_PORT=8080 make preview`.

`make site-build` usa a mesma imagem Hugo da CI, então erros de template
aparecem localmente.

## Reset

```bash
make dev/down
make dev/reset       # para, remove containers e apaga o snapshot gerado
```

Saída do Hugo (`site/public`, `site/resources`, `.hugo_build.lock`) e o snapshot
são ignorados pelo git.

## Solução de problemas

| Sintoma                              | Correção                                                         |
| ------------------------------------ | ---------------------------------------------------------------- |
| `Bind for 0.0.0.0:1314 failed`       | Outro processo usa a porta; `make dev/up SITE_PORT=1414`.        |
| Snapshot não muda                    | `make dev/logs`; verifique erros de validação do catálogo.       |
| `create output dir ... permission denied` | Use os alvos do Makefile para passar UID/GID ao Compose.    |
| Mudanças em Go são ignoradas         | `make dev/rebuild`.                                              |
| Pull da imagem Hugo falha            | Verifique a rede ou pré-baixe `hugomods/hugo:0.158.0`.            |
| Precisa do Worker localmente         | `cd cloudflare && npx wrangler dev` (fora do Compose).            |

## Gate de qualidade

Antes de commitar:

```bash
make check
```

Isso roda formatação, vet, golangci-lint, testes com race e cobertura. O hook
`pre-push` aplica o mesmo gate mais `go mod verify`.
