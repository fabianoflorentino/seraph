# seraph

> protege o que mais importa

[![CI](https://github.com/fabianoflorentino/seraph/actions/workflows/ci.yml/badge.svg)](https://github.com/fabianoflorentino/seraph/actions/workflows/ci.yml)
[![Licença: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

[English](README.md) | **Português**

**Demo:** https://seraph.fabianoflorentino.dev/

O seraph é um painel de status independente e sem anúncios para os serviços dos
quais você realmente depende: bancos, meios de pagamento, operadoras, nuvens,
mensageiros e mais. Ele lê **status pages e feeds oficiais** e publica um
snapshot JSON estático, renderizado por um painel leve em inglês e
português do Brasil.

Sem conta, sem servidor, sem banco de dados. Os favoritos são ilimitados e ficam
no seu navegador.

## Por que mais um painel de status?

Agregadores como o Downdetector dependem de relatos de usuários e scraping:
resultados atrasados, anúncios no caminho e nenhuma forma de fixar exatamente os
serviços que importam. O seraph inverte isso:

- **Fontes oficiais primeiro** — status pages e feeds oficiais, nunca scraping
  de agregadores.
- **Frescor real** — um cron confiável de cinco minutos publica um snapshot a
  cada 5–20 minutos.
- **Visão rápida** — cards de resumo, filtros de atenção/operacionais,
  categorias recolhíveis e ranking de disponibilidade de 24h quando houver
  histórico.
- **Instalável** — PWA com fallback offline; notificações ficam para depois.
- **Seu** — favoritos ilimitados por região, guardados apenas no `localStorage`,
  com exportação/importação JSON e link compartilhável para backup.
- **Simples e barato** — GitHub Pages + um Worker da Cloudflare. Custo mensal
  zero, nenhuma infraestrutura para atualizar.

## Estado

O marco **M1 (coletor)** está completo: `seraph sync` agora lê resumos do
Documentos do Atlassian Statuspage e do status.io, endpoints JSON específicos e
feeds RSS em paralelo, repete falhas transitórias e falha de forma suave por
serviço. O
catálogo inicial tem 24 serviços entre nuvem, IA, social, streaming, jogos,
e-commerce, pagamentos e mensagens, incluindo AWS e Google Cloud. Fontes
`probe` continuam como `unknown` com
nota de erro até o coletor existir.

## Privacidade e retenção

Favoritos são o único dado do usuário. Eles ficam no navegador sob
`seraph.favorites.v1`; não há conta, analytics, rastreamento ou cópia no
servidor. A exportação gera um backup JSON, e “Copiar link” cria uma URL que
contém apenas os slugs públicos dos serviços. Limpar os dados do site, usar
modo privado ou trocar de navegador/dispositivo remove a cópia local — importe
o backup ou abra o link para restaurar. Sync por conta fica deliberadamente
para uma fase paga posterior, opt-in.

## Como funciona

```
Cron Cloudflare (*/5) ──▶ GitHub Actions ──▶ seraph sync ──▶ site/static/data/state.json
                                                                    │
                                                             GitHub Pages
                                                                    │
                                                       navegador + localStorage
```

1. Um Worker da Cloudflare dispara o workflow do GitHub Actions a cada cinco
   minutos. O agendador do GitHub é best-effort e para após inatividade; o
   Worker, não.
2. A CLI valida o catálogo YAML, coleta cada serviço e grava um snapshot atômico.
3. O GitHub Pages serve o site estático; o navegador cruza o snapshot com seus
   favoritos locais e o idioma escolhido.

Mais detalhes: [docs/ARCHITECTURE.pt-br.md](docs/ARCHITECTURE.pt-br.md).

## Início rápido

Requisitos: Docker com Compose v2 e `make`. Go 1.27+ só é necessário para rodar a
CLI fora do Docker.

### Ambiente completo

```bash
git clone https://github.com/fabianoflorentino/seraph.git
cd seraph

make dev/up     # Hugo com live reload + loop de coleta
# abra http://localhost:1314
```

O site recarrega enquanto você edita e um container atualiza o snapshot a cada
minuto. `make dev/logs` acompanha os dois serviços; `make dev/down` para tudo.
Veja [docs/DEVELOPMENT.pt-br.md](docs/DEVELOPMENT.pt-br.md) para portas,
overrides e solução de problemas.

### Apenas a CLI

```bash
make validate   # valida o catálogo, sem rede
make sync       # grava site/static/data/state.json
make preview    # compila e serve em segundo plano em http://localhost:8000
make preview/logs   # acompanha requisições e erros
make preview/stop   # para o servidor de preview
```

`make preview` retorna imediatamente. Os logs ficam em `.preview/server.log`;
para outra porta use `make preview PREVIEW_PORT=8080`.

Prefere Go puro para a CLI?

```bash
go run ./cmd/seraph validate --catalog catalog/packs
go run ./cmd/seraph sync --catalog catalog/packs
make site-build
python3 -m http.server -d site/public 8000
```

## CLI

| Comando    | Propósito                                          |
| ---------- | -------------------------------------------------- |
| `version`  | Mostra versão, commit e data do build.             |
| `validate` | Valida todos os packs do catálogo; sem rede.       |
| `sync`     | Coleta os serviços e grava o snapshot.             |

Flags globais: `--config <path>`, `--log-level <level>`. `sync` e `validate`
aceitam `--catalog <dir>` (padrão `catalog/packs`).

```bash
seraph --help
seraph validate --catalog catalog/packs
seraph sync --catalog catalog/packs --log-level debug
```

## Configuração

`config/seraph.yml` é opcional; os padrões valem quando ele não existe. A
precedência é **flag > ambiente > arquivo > padrão**.

| Chave         | Env                  | Padrão      |
| ------------- | -------------------- | ----------- |
| `out_dir`     | `SERAPH_OUT_DIR`     | `site/static/data` |
| `base_url`    | `SERAPH_BASE_URL`    | vazio       |
| `timeout`     | `SERAPH_TIMEOUT`     | `8s`        |
| `concurrency` | `SERAPH_CONCURRENCY` | `12`        |
| `retries`     | `SERAPH_RETRIES`     | `2`         |
| `log_level`   | `SERAPH_LOG_LEVEL`   | `info`      |

Veja [docs/OPERATIONS.pt-br.md](docs/OPERATIONS.pt-br.md) para restrições e
notas de operação.

## Catálogo

Serviços são YAML, revisados como código:

```yaml
version: 1
services:
  - slug: github
    name: GitHub
    category: cloud
    official_url: https://github.com
    status_url: https://www.githubstatus.com
    regions:
      global: primary
    source:
      type: statuspage
      url: https://www.githubstatus.com/api/v2/summary.json
```

Os packs ficam em `catalog/packs/`. Para adicionar um serviço sem editar YAML:

```bash
make catalog-add CATALOG_SLUG=example CATALOG_NAME=Example CATALOG_CATEGORY=cloud \
  CATALOG_OFFICIAL_URL=https://example.com CATALOG_STATUS_URL=https://status.example.com \
  CATALOG_SOURCE_TYPE=statuspage CATALOG_SOURCE_URL=https://status.example.com/api/v2/summary.json
```

O comando valida todos os packs antes de gravar e deixa o arquivo intacto em
caso de erro. O schema reconhece `statuspage`, `statusio`, `rss`, `json` e
`probe`; hoje `statuspage`, `statusio`, `rss` e `json` estão implementados. As
regiões são
`global` mais códigos ISO 3166-1 alpha-2. O schema completo, as regras de
validação e o checklist para adicionar um serviço estão em
[docs/CATALOG.pt-br.md](docs/CATALOG.pt-br.md).

## Implantação

1. Crie o repositório público e habilite o Pages com origem *GitHub Actions*.
2. Publique o despachante em `cloudflare/` com os segredos `GITHUB_TOKEN`
   (Actions: write) e `TRIGGER_SECRET`.
3. Faça push em `main`; o workflow compila, valida, coleta e publica.

Passo a passo, monitoramento e solução de problemas:
[docs/OPERATIONS.pt-br.md](docs/OPERATIONS.pt-br.md).

## Desenvolvimento

```bash
make check      # fmt, vet, lint, testes com race e cobertura
make test       # race + shuffle
make lint       # golangci-lint
lefthook install
```

- Ambiente local: `make dev/up`, `dev/logs`, `dev/down`, `dev/rebuild`,
  `dev/collect`, `dev/reset`; veja
  [docs/DEVELOPMENT.pt-br.md](docs/DEVELOPMENT.pt-br.md).
- O site é um projeto Hugo em `site/`; `make site-build` gera `site/public` com
  a mesma imagem Hugo usada na CI.
- Piso de cobertura: **80%** (garantido pelo lefthook e pela CI).
- Código, identificadores, comentários e docs técnicos em **inglês**; o site e a
  documentação de usuário em inglês e português do Brasil.
- Convenções para contribuidores e agentes de IA: [AGENTS.md](AGENTS.md).

## Roadmap

- **M1 (concluído)** — coletor `statuspage`, retentativas e erro por serviço.
- **M2** — primeiro serviço brasileiro, exercitando regiões de ponta a ponta.
- **M3** — coletor `probe` para endpoints oficiais de saúde.
- **M4** — ampliar o catálogo para ~50 serviços, Brasil + global.
- **Histórico (concluído)** — retenção de 24h e ranking de disponibilidade.
- **PWA (concluído)** — manifest, ícones, service worker e shell offline.
- **Depois** — notificações PWA e comparação de latência por região.

## Licença

MIT. Veja [LICENSE](LICENSE).
