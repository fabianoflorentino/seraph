# seraph

> protege o que mais importa

[![CI](https://github.com/fabianoflorentino/seraph/actions/workflows/ci.yml/badge.svg)](https://github.com/fabianoflorentino/seraph/actions/workflows/ci.yml)
[![Licença: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

[English](README.md) | **Português**

O seraph é um painel de status independente e sem anúncios para os serviços dos
quais você realmente depende: bancos, meios de pagamento, operadoras, nuvens,
mensageiros e mais. Ele lê **status pages oficiais e sondagens HTTP diretas** e
publica um snapshot JSON estático, renderizado por um painel leve em inglês e
português do Brasil.

Sem conta, sem servidor, sem banco de dados. Os favoritos são ilimitados e ficam
no seu navegador.

## Por que mais um painel de status?

Agregadores como o Downdetector dependem de relatos de usuários e scraping:
resultados atrasados, anúncios no caminho e nenhuma forma de fixar exatamente os
serviços que importam. O seraph inverte isso:

- **Fontes oficiais primeiro** — status pages e sondagens, nunca scraping de
  agregadores.
- **Frescor real** — um cron confiável de cinco minutos publica um snapshot a
  cada 5–20 minutos.
- **Seu** — favoritos ilimitados por região, guardados apenas no `localStorage`.
- **Simples e barato** — GitHub Pages + um Worker da Cloudflare. Custo mensal
  zero, nenhuma infraestrutura para atualizar.

## Estado

O marco **M0 (esqueleto)** está completo: CLI, validação de catálogo, schema do
snapshot, site bilíngue, despachante na Cloudflare, CI e documentação. Os
coletores são o próximo marco; até lá, `seraph sync` marca todo serviço como
`unknown`, permitindo exercitar o fluxo de ponta a ponta.

## Como funciona

```
Cron Cloudflare (*/5) ──▶ GitHub Actions ──▶ seraph sync ──▶ site/data/state.json
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

Requisitos: Go 1.27+, `make` e Python 3 (para o servidor de pré-visualização).

```bash
git clone https://github.com/fabianoflorentino/seraph.git
cd seraph

make validate   # valida o catálogo, sem rede
make sync       # grava site/data/state.json
make preview    # http://localhost:8000
```

Prefere Go puro?

```bash
go run ./cmd/seraph validate --catalog catalog/packs
go run ./cmd/seraph sync --catalog catalog/packs
python3 -m http.server -d site 8000
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
| `out_dir`     | `SERAPH_OUT_DIR`     | `site/data` |
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
    regions:
      global: primary
    source:
      type: statuspage
      url: https://www.githubstatus.com/api/v2/summary.json
```

Os packs ficam em `catalog/packs/`. Os tipos de fonte são `statuspage`,
`statusio`, `rss` e `probe`; as regiões são `global` mais códigos ISO 3166-1
alpha-2. O schema completo, as regras de validação e o checklist para adicionar
um serviço estão em [docs/CATALOG.pt-br.md](docs/CATALOG.pt-br.md).

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

- Piso de cobertura: **80%** (garantido pelo lefthook e pela CI).
- Código, identificadores, comentários e docs técnicos em **inglês**; o site e a
  documentação de usuário em inglês e português do Brasil.
- Convenções para contribuidores e agentes de IA: [AGENTS.md](AGENTS.md).

## Roadmap

- **M1** — coletor `statuspage` e tratamento de erro por serviço.
- **M2** — primeiro serviço brasileiro, exercitando regiões de ponta a ponta.
- **M3** — coletor `probe` para serviços sem status page.
- **M4** — catálogo completo (~50 serviços, Brasil + global).
- **Depois** — histórico, PWA instalável com notificações.

## Licença

MIT. Veja [LICENSE](LICENSE).
