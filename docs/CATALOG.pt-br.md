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
    status_url: https://www.githubstatus.com
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
| `official_url` | não         | URL HTTPS absoluta do site oficial; link de fallback do card.           |
| `status_url`   | não         | URL HTTPS absoluta da status page; usada como link do card.             |
| `regions`      | sim         | Mapa código de região → nível. Ao menos uma entrada e um `primary`.     |
| `source`       | sim         | Endpoint de observação; veja abaixo.                                    |

Campos desconhecidos são rejeitados (`KnownFields(true)`), o que pega erros como
`official-url`.

## Categorias

`banking`, `payments`, `telecom`, `messaging`, `social`, `ai`, `cloud`,
`streaming`, `gaming`, `transport`, `ecommerce`, `gov`.

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
ficar selecionável. O filtro fica oculto enquanto só existe uma região. Nomes
localizados de `global` e `BR` vêm dos arquivos i18n; demais códigos usam
`Intl.DisplayNames`.

## Fontes

Apenas `statuspage` está implementado. `statusio`, `rss` e `probe` passam na
validação do schema, mas ficam `unknown` com erro "not implemented" até seus
coletores existirem.

| Tipo         | Lê                                                | Mapeamento de status                                                |
| ------------ | ------------------------------------------------- | ------------------------------------------------------------------- |
| `statuspage` | `summary.json` do Atlassian Statuspage            | `none` → operacional; `minor` → degradado; `major`/`critical` → queda; `maintenance` → degradado. |
| `statusio`   | Documento de status do status.io                   | `100` → operacional; `200`/`300`/`400`/`600` → degradado; `500` → queda. |
| `rss`        | Feed de incidentes                                 | Feed vazio → operacional; qualquer item → degradado.                |
| `json`       | JSON específico do serviço (`format` obrigatório)  | `slack` ou `stripe`; veja abaixo.                                   |
| `probe`      | Requisição HTTPS direta para `url` (planejado)    | 2xx → operacional; outro/erro → degradado ou desconhecido.          |

### `statuspage`

O Atlassian Statuspage expõe `https://<host>/api/v2/summary.json`. Páginas do
incident.io servem o mesmo payload, então usam esse tipo de fonte também. O
campo opcional `component` limita o status a um componente. Os estados do componente
mapeiam assim: `operational` → operacional; `degraded_performance`,
`partial_outage` e `under_maintenance` → degradado; `major_outage` → queda.
Componente ausente ou estado desconhecido gera `unknown` com nota de erro.

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

Feed vazio significa operacional; qualquer item significa degradado. Use apenas
feeds que publicam incidentes atuais, não histórico.

```yaml
source:
  type: rss
  url: https://status.example.com/feed/
```

### `json`

Documentos JSON específicos de cada serviço. `format` é obrigatório e hoje
aceita `slack`, `stripe`, `aws` e `gcp`.

```yaml
source:
  type: json
  format: slack
  url: https://status.slack.com/api/v2.0.0/current
```

- `slack`: `status: ok` sem incidentes ativos → operacional; incidentes ativos
  ou `notice`/`degraded` → degradado; `critical`/`major`/`outage` → queda.
- `stripe`: `largestatus` `up` → operacional; `degraded` → degradado;
  `down`/`outage` → queda. Sem `largestatus`, vale o pior status dos serviços.
- `aws`: lê o `data.json` em UTF-16; eventos resolvidos são ignorados e eventos
  abertos/em andamento (`1`/`2`) marcam o serviço como degradado.
- `gcp`: lê `incidents.json`; incidente sem `end` ou com `end` futuro marca o
  serviço como degradado.

### `probe`

A sondagem é o recurso quando o serviço expõe um endpoint oficial de saúde, mas
não tem status page. É intencionalmente superficial: DNS, TLS, status HTTP e
latência. Quando implementada, use apenas para um endpoint oficial de saúde,
nunca para uma home page de marketing.

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
- URLs HTTPS absolutas para `source.url`, `official_url` e `status_url`.
- `source.format` obrigatório para `json`; `slack`, `stripe`, `aws` e `gcp` são
  aceitos.

Erros são agregados: uma execução reporta todos os problemas com arquivo, slug e
campo. Exemplo:

```
br.yaml: nubank.category: unsupported category "fintech"
```

## Adicionando um serviço

1. Escolha o pack: `global.yaml` para serviços mundiais, um pack regional como
   `br.yaml` para locais.
2. Prefira uma status page ou feed oficial. Use o adapter que corresponde ao
   provedor; nunca faça scraping de HTML ou de uma home page de marketing.
3. Adicione a entrada com um slug estável e legível:

   ```bash
   make catalog-add CATALOG_PACK=catalog/packs/global.yaml CATALOG_SLUG=example \
     CATALOG_NAME=Example CATALOG_CATEGORY=cloud \
     CATALOG_STATUS_URL=https://status.example.com CATALOG_SOURCE_TYPE=statuspage \
     CATALOG_SOURCE_URL=https://status.example.com/api/v2/summary.json
   ```

   O comando valida todos os packs e deixa o arquivo intacto em caso de erro.
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
