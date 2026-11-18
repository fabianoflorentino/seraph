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
- **Fontes oficiais primeiro.** Apenas status pages e feeds oficiais.
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
                                                │ site/static/data/state.json
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
| Coletores    | `internal/collector`         | Observações de status page, status.io, JSON e RSS. |
| Site         | `site/`                      | Site Hugo: painel, i18n, favoritos.               |
| Dispatcher   | `cloudflare/`                | Cron de cinco minutos → `workflow_dispatch`.      |
| CI/CD        | `.github/workflows/`          | Lint, testes, cobertura, build, deploy do Pages.  |

## Fluxo de dados

1. O Worker da Cloudflare dispara a cada cinco minutos e chama a API do GitHub
   para executar `dashboard.yml` (também manual ou em push).
2. O workflow compila a CLI, valida o catálogo e roda `seraph sync`.
3. O `sync` carrega todos os packs em `catalog/packs`, resolve a fonte de cada
   serviço, coleta observações e monta um snapshot.
4. O snapshot é gravado atomicamente em `site/static/data/state.json`.
5. O Hugo renderiza `site/` em `site/public`, enviado como artefato e
   publicado no Pages.
6. O navegador busca `data/state.json` com cache-busting, cruza com os favoritos
   locais e renderiza o painel em inglês ou português.

## Mapa de pacotes

| Caminho              | Propósito                                                    |
| -------------------- | ------------------------------------------------------------ |
| `cmd/seraph`         | Ponto de entrada; imprime erros e define o código de saída.  |
| `internal/cmd`       | `version`, `sync`, `validate`; helpers de config e logger.   |
| `internal/config`    | Configuração tipada com validação e overrides de ambiente.   |
| `internal/catalog`   | Tipos do catálogo, regras de validação, carregamento de packs. |
| `internal/collector` | Cliente Statuspage, retentativas, concorrência, mapeamento.   |
| `internal/state`     | Tipos do snapshot, persistência atômica, versão de schema.   |
| `internal/version`   | Metadados de build via `-ldflags`.                           |
| `catalog/packs`      | Definições YAML agrupadas por público/região.                |
| `config`             | `seraph.yml` padrão.                                         |
| `site`               | Site Hugo: layouts, assets estáticos, i18n, snapshot.         |
| `cloudflare`         | Worker despachante de cron.                                  |

Dependências são injetadas por construtores; não há estado mutável global.
Coletores aceitam `*http.Client` e um relógio para testes sem rede.

## Desenvolvimento local

O `docker-compose.yml` sobe dois serviços. `site` inicia a imagem Hugo fixada em
modo servidor em <http://localhost:1314> com live reload, montando `site/`.
`collector` compila o `Dockerfile.dev` e valida/sincroniza a cada minuto no
diretório montado `site/static/data`, então o painel mostra o loop completo sem
publicar. Detalhes e overrides: [DEVELOPMENT.pt-br.md](DEVELOPMENT.pt-br.md).

## Contrato do snapshot

`schema_version` é independente da versão da aplicação. Mudanças incompatíveis
incrementam o número; leitores rejeitam versões desconhecidas com
`ErrUnsupportedSchema`. Cada serviço traz status, regiões, horário da
observação, latência opcional, erro opcional, `official_url` e `status_url`
opcionais, além de um histórico curto. O painel usa `status_url` como link do
card quando presente, com fallback para `official_url`. Trata todo campo como
opcional, exceto `slug`, `name`, `category`, `regions` e `status`.

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

1. `internal/collector`: implementações status.io, RSS e probe.
2. Retenção de histórico entre execuções (merge do `state.json` anterior).
3. PWA instalável com notificações locais para favoritos.
4. Comparação de latência por região (múltiplos locais de sondagem).
