# Referência do catálogo

[English](CATALOG.md) | Português

O catálogo é um conjunto de packs YAML em `catalog/packs/`. Cada pack é um
arquivo de serviços, normalmente agrupado por público ou região. Packs são
dados: adicionar um serviço é uma mudança YAML revisada, não código.

## Formato do pack

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

### Campos do pack

| Campo      | Obrigatório | Regras                                                     |
| ---------- | ----------- | ---------------------------------------------------------- |
| `version`  | sim         | Precisa ser `1`. Versões desconhecidas falham na validação. |
| `services` | sim         | Lista de serviços; duplicatas no pack falham.              |

### Campos do serviço

| Campo          | Obrigatório | Regras                                                                  |
| -------------- | ----------- | ----------------------------------------------------------------------- |
| `slug`         | sim         | Kebab-case minúsculo (`^[a-z0-9]+(-[a-z0-9]+)*$`), único em todos os packs. |
| `name`         | sim         | Nome exibido, 1–80 caracteres.                                          |
| `category`     | sim         | Uma das categorias abaixo.                                              |
| `official_url` | não         | URL HTTPS absoluta, usada como link do card.                            |
| `regions`      | sim         | Mapa código de região → nível. Ao menos uma entrada e um `primary`.     |
| `source`       | sim         | Endpoint de observação; veja abaixo.                                    |

Campos desconhecidos são rejeitados (`KnownFields(true)`), o que pega erros como
`official-url`.

## Categorias

`banking`, `payments`, `telecom`, `messaging`, `social`, `cloud`, `streaming`,
`gaming`, `transport`, `ecommerce`, `gov`.

O painel agrupa por categoria nessa ordem. Adicionar categoria exige mudanças em
`internal/catalog` e `site/i18n/*.js`.

## Regiões

| Código   | Significado                                                  |
| -------- | ------------------------------------------------------------ |
| `global` | Serviço mundial, relevante para todas as regiões (reservado). |
| `BR`     | Brasil; códigos ISO 3166-1 alpha-2 são maiúsculos.           |

Níveis:

| Nível       | Significado                                                    |
| ----------- | -------------------------------------------------------------- |
| `primary`   | Exibido por padrão ao selecionar a região.                     |
| `secondary` | Mundial ou parcialmente relevante; exibido ao expandir.        |

Um serviço pode pertencer a várias regiões:

```yaml
regions:
  global: primary
  BR: secondary
```

Os botões de região vêm do snapshot, então basta um novo código ISO no pack para
ficar selecionável. Nomes localizados de `global` e `BR` vêm dos arquivos i18n;
demais códigos usam `Intl.DisplayNames`.

## Fontes

| Tipo         | Lê                                                | Mapeamento de status                                                |
| ------------ | ------------------------------------------------- | ------------------------------------------------------------------- |
| `statuspage` | `summary.json` do Atlassian Statuspage            | `none` → operacional; `minor` → degradado; `major`/`critical` → queda. |
| `statusio`   | Documento de status do status.io                  | Status do componente e severidade do incidente.                     |
| `rss`        | Feed de incidentes                                | Incidente ativo → degradado; feed inacessível → desconhecido.       |
| `probe`      | Requisição HTTPS direta para `url`                | 2xx → operacional; outro/erro → degradado ou desconhecido.          |

### `statuspage`

O Atlassian Statuspage expõe `https://<host>/api/v2/summary.json`. O campo
opcional `component` limita o status a um componente:

```yaml
source:
  type: statuspage
  url: https://www.githubstatus.com/api/v2/summary.json
  component: GitHub
```

### `statusio`

Use a URL do documento de status, não a página HTML:

```yaml
source:
  type: statusio
  url: https://status.example.com/1.0/status/abc123
```

### `rss`

```yaml
source:
  type: rss
  url: https://status.example.com/history.rss
```

### `probe`

A sondagem é o recurso quando não existe status page oficial. É intencionalmente
superficial: DNS, TLS, status HTTP e latência.

```yaml
source:
  type: probe
  url: https://www.example.com/health
```

## Regras de validação

`seraph validate` verifica, sem acesso à rede:

- `version` suportada (hoje `1`).
- Sintaxe e unicidade do slug — no pack e entre packs.
- Nome não vazio de até 80 caracteres.
- Categoria conhecida.
- Códigos de região: `global` ou duas letras maiúsculas.
- Níveis válidos e ao menos uma região `primary`.
- Tipo de fonte conhecido.
- URLs HTTPS absolutas para `source.url` e `official_url`.

Erros são agregados: uma execução reporta todos os problemas com arquivo, slug e
campo. Exemplo:

```
br.yaml: nubank.category: unsupported category "fintech"
```

## Adicionando um serviço

1. Escolha o pack: `global.yaml` para serviços mundiais, um pack regional como
   `br.yaml` para locais.
2. Prefira uma status page oficial. Use `probe` apenas quando não existir.
3. Adicione a entrada com um slug estável e legível.
4. Rode `go run ./cmd/seraph validate --catalog catalog/packs`.
5. Rode `go run ./cmd/seraph sync --catalog catalog/packs`, depois
   `make preview` e confira o card.
6. Cite a fonte no pull request. Serviços são adicionados deliberadamente, não
   em massa.

## Remoção e renomeação

- Remover um serviço é uma edição normal; favoritos que o referenciavam apenas
  deixam de casar.
- Evite renomear slugs: favoritos usam o slug como chave. Se for inevitável,
  registre em `CHANGELOG.md`.
- Nunca reutilize um slug para outro serviço.
