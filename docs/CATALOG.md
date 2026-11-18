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
    status_url: https://www.githubstatus.com
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
| `official_url` | no       | Absolute HTTPS URL for the official site; fallback card link.         |
| `status_url`   | no       | Absolute HTTPS URL for the status page; used as the card link.        |
| `regions`      | yes      | Map of region code → level. At least one entry, at least one `primary`. |
| `source`       | yes      | Observation endpoint; see below.                                      |

Unknown fields are rejected (`KnownFields(true)`), which catches typos such as
`official-url`.

## Categories

`banking`, `payments`, `telecom`, `messaging`, `social`, `ai`, `cloud`,
`streaming`, `gaming`, `transport`, `ecommerce`, `gov`.

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
ISO code to a pack is enough to make it selectable. The filter is hidden while
only one region exists. Localized names for `global` and `BR` come from the i18n
files; other codes use `Intl.DisplayNames`.

## Sources

`statuspage`, `statusio`, `rss`, and `json` are implemented. `probe` passes
schema validation but stays `unknown` with a "not implemented" error until its
collector lands.

| Type         | Reads                                             | Status mapping                                                     |
| ------------ | ------------------------------------------------- | ------------------------------------------------------------------ |
| `statuspage` | Atlassian Statuspage `summary.json`               | `none` → operational; `minor` → degraded; `major`/`critical` → major; `maintenance` → degraded. |
| `statusio`   | status.io status document                          | `100` → operational; `200`/`300`/`400`/`600` → degraded; `500` → major. |
| `rss`        | Incident feed                                      | Empty feed → operational; any item → degraded.                      |
| `json`       | Service-specific JSON (`format` required)          | `slack` or `stripe`; see below.                                     |
| `probe`      | Direct HTTPS request to `url` (planned)           | 2xx → operational; other/error → degraded or unknown.              |

### `statuspage`

Atlassian Statuspage exposes `https://<host>/api/v2/summary.json`. incident.io
status pages serve the same payload, so they use this source type as well.
Optional `component` scopes the status to one component name. Component states map as:
`operational` → operational; `degraded_performance`, `partial_outage`, and
`under_maintenance` → degraded; `major_outage` → major. A missing component or
an unrecognized state yields `unknown` with an error note.

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

An empty feed means operational; any item means degraded. Use only feeds that
publish current incidents, not history.

```yaml
source:
  type: rss
  url: https://status.example.com/feed/
```

### `json`

Service-specific JSON documents. `format` is required and currently supports
`slack`, `stripe`, `aws`, and `gcp`.

```yaml
source:
  type: json
  format: slack
  url: https://status.slack.com/api/v2.0.0/current
```

- `slack`: `status: ok` with no active incidents → operational; active
  incidents or `notice`/`degraded` → degraded; `critical`/`major`/`outage` →
  major.
- `stripe`: `largestatus` `up` → operational; `degraded` → degraded;
  `down`/`outage` → major. Without `largestatus`, the worst service status wins.
- `aws`: reads the UTF-16 `data.json` event feed; resolved events are ignored,
  and open/in-progress events (`1`/`2`) mark the service degraded.
- `gcp`: reads `incidents.json`; an incident without `end` or with a future
  `end` marks the service degraded.

### `probe`

A probe is the fallback when a service exposes an official health endpoint but
no status page. It is intentionally shallow: DNS, TLS, HTTP status, and latency.
When implemented, use it only for an official health endpoint, never for a
marketing home page.

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
- Absolute HTTPS URLs for `source.url`, `official_url`, and `status_url`.
- `source.format` required for `json`; `slack`, `stripe`, `aws`, and `gcp` are
  accepted.

Errors are aggregated: a single run reports every problem with file, slug, and
field. Example:

```
br.yaml: nubank.category: unsupported category "fintech"
```

## Adding a service

1. Pick the pack: `global.yaml` for worldwide services, a region pack such as
   `br.yaml` for local ones.
2. Prefer an official status page or feed. Use `seraph catalog inspect <domain>`
   to probe known shapes and print a draft; review it manually. Use the adapter
   that matches the provider; never scrape HTML or a marketing page.
3. Add the entry with a stable, human-readable slug:

   ```bash
   make catalog-add CATALOG_PACK=catalog/packs/global.yaml CATALOG_SLUG=example \
     CATALOG_NAME=Example CATALOG_CATEGORY=cloud \
     CATALOG_STATUS_URL=https://status.example.com CATALOG_SOURCE_TYPE=statuspage \
     CATALOG_SOURCE_URL=https://status.example.com/api/v2/summary.json
   ```

   The command validates every pack and leaves the file untouched on error.
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
