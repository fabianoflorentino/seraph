# Architecture

[English](ARCHITECTURE.md) | Português

## Objetivos

O seraph é um painel de status barato e confiável. Ele responde uma pergunta —
"o serviço de que eu dependo está no ar agora?" — usando fontes oficiais em vez
de scraping de agregadores, e custa zero para operar.

Restrições de projeto:

- **Zero infraestrutura.** JSON estático no GitHub Pages, sem servidor ou banco.
- **Frescor real.** Um snapshot a cada 5–20 minutos, com cron confiável.
- **Favoritos ilimitados.** Guardados no navegador; sem contas, sem rastreio.
- **Fontes oficiais primeiro.** Apenas status pages e sondagens HTTP diretas.
- **Operação entediante.** Se o GitHub cair, o último snapshot continua servindo.

## Componentes

```
┌─────────────────────────┐        ┌──────────────────────────┐
│ Cron Cloudflare Worker  │──────▶ │ GitHub Actions           │
│ seraph-dispatch */5 * * │       │ workflow_dispatch        │
└─────────────────────────┘        └────────────┬─────────────┘
                                                │
                 ┌──────────────────────────────▼────────────────┐
                 │ CLI seraph                                    │
                 │ catalog.Load → coleta → state.WriteAtomic     │
                 └──────────────────────────────┬────────────────┘
                                                │ site/data/state.json
                                                ▼
                                      ┌───────────────────┐
                                      │ GitHub Pages      │
                                      │ painel estático   │
                                      └───────────────────┘
```

| Componente   | Local                        | Responsabilidade                                  |
| ------------ | ---------------------------- | ------------------------------------------------- |
| CLI          | `cmd/seraph`, `internal/cmd` | Árvore de comandos, config, logs.                 |
| Catálogo     | `internal/catalog`           | Carregar, validar e mesclar packs YAML.           |
| Estado       | `internal/state`             | Schema do snapshot, escrita atômica, leitura.     |
| Config       | `internal/config`            | Arquivo + ambiente `SERAPH_*` + flags.            |
| Coletores    | planejado (`internal/collector`) | Status pages e sondagens (roadmap).           |
| Site         | `site/`                      | Painel estático, i18n, favoritos.                 |
| Dispatcher   | `cloudflare/`                | Cron de cinco minutos → `workflow_dispatch`.      |
| CI/CD        | `.github/workflows/`          | Lint, testes, cobertura, build, deploy do Pages.  |

## Fluxo de dados

1. O Worker da Cloudflare dispara a cada cinco minutos e chama a API do GitHub
   para executar `dashboard.yml` (também manual ou em push).
2. O workflow compila a CLI, valida o catálogo e roda `seraph sync`.
3. O `sync` carrega todos os packs em `catalog/packs`, resolve a fonte de cada
   serviço, coleta observações e monta um snapshot.
4. O snapshot é gravado atomicamente em `site/data/state.json`.
5. O diretório `site/` é enviado como artefato e publicado no Pages.
6. O navegador busca `data/state.json` com cache-busting, cruza com os favoritos
   locais e renderiza o painel em inglês ou português.

## Mapa de pacotes

| Caminho              | Propósito                                                    |
| -------------------- | ------------------------------------------------------------ |
| `cmd/seraph`         | Ponto de entrada; imprime erros e define o código de saída.  |
| `internal/cmd`       | `version`, `sync`, `validate`; helpers de config e logger.   |
| `internal/config`    | Configuração tipada com validação e overrides de ambiente.   |
| `internal/catalog`   | Tipos do catálogo, regras de validação, carregamento de packs. |
| `internal/state`     | Tipos do snapshot, persistência atômica, versão de schema.   |
| `internal/version`   | Metadados de build via `-ldflags`.                           |
| `catalog/packs`      | Definições YAML agrupadas por público/região.                |
| `config`             | `seraph.yml` padrão.                                         |
| `site`               | Painel estático (sem etapa de build).                        |
| `cloudflare`         | Worker despachante de cron.                                  |

Dependências são injetadas por construtores; não há estado mutável global.
Coletores receberão `*http.Client` e um relógio para testes sem rede.

## Contrato do snapshot

`schema_version` é independente da versão da aplicação. Mudanças incompatíveis
incrementam o número; leitores rejeitam versões desconhecidas com
`ErrUnsupportedSchema`. Cada serviço traz status, regiões, horário da
observação, latência opcional, erro opcional e um histórico curto. O painel
trata todo campo como opcional, exceto `slug`, `name`, `category`, `regions` e
`status`.

Favoritos **não** entram no snapshot. Eles ficam no `localStorage` sob chaves
`seraph.*.v1`, mantendo o artefato público anônimo e cacheável.

## Escolhas de confiabilidade

- **Escrita atômica:** arquivo temporário no diretório de destino e `rename`,
  para leitores nunca verem um snapshot parcial.
- **Falha suave por serviço:** uma fonte quebrada não bloqueia a execução; vira
  `unknown` com uma nota de erro.
- **Catálogo é dado:** adicionar serviço é uma mudança YAML revisada como código.
- **Validação antes da coleta:** erros de schema falham cedo, com arquivo e
  campo na mensagem.
- **Sem segredos no repositório:** token do GitHub e segredo de trigger ficam na
  Cloudflare; o workflow usa o `GITHUB_TOKEN` padrão.

## Roadmap

1. `internal/collector`: implementações statuspage, status.io, RSS e probe.
2. Retenção de histórico entre execuções (merge do `state.json` anterior).
3. PWA instalável com notificações locais para favoritos.
4. Comparação de latência por região (múltiplos locais de sondagem).
