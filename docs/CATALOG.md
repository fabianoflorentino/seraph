# Catalog reference

English | [Português](CATALOG.pt-br.md)

The catalog is a set of YAML packs under `catalog/packs/`. Each pack is one
file of services, usually grouped by audience or region. Packs are data: adding
a service is a reviewed YAML change, not a code change.

## Pack format

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

### Pack fields

| Field      | Required | Rules                                                      |
| ---------- | -------- | ---------------------------------------------------------- |
| `version`  | yes      | Must be `1`. Unknown versions fail validation.             |
| `services` | yes      | List of service entries; duplicates within a pack fail.    |

### Service fields

| Field          | Required | Rules                                                                 |
| -------------- | -------- | --------------------------------------------------------------------- |
| `slug`         | yes      | Lower-kebab-case (`^[a-z0-9]+(-[a-z0-9]+)*$`), unique across all packs. |
| `name`         | yes      | Display name, 1–80 characters.                                        |
| `category`     | yes      | One of the categories below.                                          |
| `official_url` | no       | Absolute HTTPS URL, shown as the card link.                           |
| `regions`      | yes      | Map of region code → level. At least one entry, at least one `primary`. |
| `source`       | yes      | Observation endpoint; see below.                                      |

Unknown fields are rejected (`KnownFields(true)`), which catches typos such as
`official-url`.

## Categories

`banking`, `payments`, `telecom`, `messaging`, `social`, `cloud`, `streaming`,
`gaming`, `transport`, `ecommerce`, `gov`.

The dashboard groups services by category in that order. Adding a category
requires code changes in `internal/catalog` and `site/i18n/*.js`.

## Regions

| Code     | Meaning                                                    |
| -------- | ---------------------------------------------------------- |
| `global` | Worldwide service, relevant to every region (reserved).    |
| `BR`     | Brazil; ISO 3166-1 alpha-2 codes are uppercase.            |

Levels:

| Level       | Meaning                                                        |
| ----------- | -------------------------------------------------------------- |
| `primary`   | Shown by default when the region is selected.                  |
| `secondary` | Worldwide or partially relevant; shown only when expanded.     |

A service may belong to several regions:

```yaml
regions:
  global: primary
  BR: secondary
```

Region buttons in the dashboard are derived from the snapshot, so adding a new
ISO code to a pack is enough to make it selectable. Localized names for `global`
and `BR` come from the i18n files; other codes use `Intl.DisplayNames`.

## Sources

| Type         | Reads                                             | Status mapping                                                     |
| ------------ | ------------------------------------------------- | ------------------------------------------------------------------ |
| `statuspage` | Atlassian Statuspage `summary.json`               | `none` → operational; `minor` → degraded; `major`/`critical` → major. |
| `statusio`   | status.io status document                          | Component status and incident severity.                            |
| `rss`        | Incident feed                                     | Active incident → degraded; unreachable feed → unknown.            |
| `probe`      | Direct HTTPS request to `url`                      | 2xx → operational; other/error → degraded or unknown.              |

### `statuspage`

Atlassian Statuspage exposes `https://<host>/api/v2/summary.json`. Optional
`component` scopes the status to one component name:

```yaml
source:
  type: statuspage
  url: https://www.githubstatus.com/api/v2/summary.json
  component: GitHub
```

### `statusio`

Use the status document URL, not the HTML page:

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

A probe is the fallback when no official status page exists. It is intentionally
shallow: DNS, TLS, HTTP status, and latency.

```yaml
source:
  type: probe
  url: https://www.example.com/health
```

## Validation rules

`seraph validate` enforces, without network access:

- Supported `version` (currently `1`).
- Slug syntax and uniqueness — within a pack and across packs.
- Non-empty name of at most 80 characters.
- Known category.
- Region codes: `global` or two uppercase letters.
- Valid levels, and at least one `primary` region.
- Known source type.
- Absolute HTTPS URLs for `source.url` and `official_url`.

Errors are aggregated: a single run reports every problem with file, slug, and
field. Example:

```
br.yaml: nubank.category: unsupported category "fintech"
```

## Adding a service

1. Pick the pack: `global.yaml` for worldwide services, a region pack such as
   `br.yaml` for local ones.
2. Prefer an official status page. Use `probe` only when none exists.
3. Add the entry with a stable, human-readable slug.
4. Run `go run ./cmd/seraph validate --catalog catalog/packs`.
5. Run `go run ./cmd/seraph sync --catalog catalog/packs`, then
   `make preview` and check the card.
6. Mention the source in the pull request. Services are added deliberately, not
   in bulk.

## Removal and renaming

- Removing a service is a normal catalog edit; favorites referencing it simply
  stop matching.
- Avoid renaming slugs: favorites are keyed by slug. If a rename is unavoidable,
  note it in `CHANGELOG.md`.
- Never reuse a slug for a different service.
