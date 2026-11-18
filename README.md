# seraph

> protects that which matters most

[![CI](https://github.com/fabianoflorentino/seraph/actions/workflows/ci.yml/badge.svg)](https://github.com/fabianoflorentino/seraph/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**English** | [Português](README.pt-br.md)

**Demo:** https://seraph.fabianoflorentino.dev/

seraph is an independent, ad-free status board for the services you actually
depend on: banks, payment networks, carriers, clouds, messaging apps, and more.
It reads **official status pages and feeds**, then publishes a
static JSON snapshot that a lightweight dashboard renders in English and
Brazilian Portuguese.

No account, no database. Favorites are unlimited and live in your browser.
The same build ships as a public static site or as a self-contained container.

## Why another status board?

Aggregators like Downdetector rely on user reports and page scraping: results
lag, ads get in the way, and there is no way to pin exactly the services you
care about. seraph inverts that:

- **Official signals first** — official status pages and feeds, never scraped
  aggregators.
- **Real freshness** — a reliable five-minute cron publishes a snapshot every
  5–20 minutes.
- **At a glance** — overview cards, operational/attention/outage filters,
  collapsible categories, and a 24-hour availability ranking once history
  builds up.
- **Installable** — PWA shell with an offline fallback; notifications come
  later.
- **Yours** — unlimited favorites and named collections per region, stored only
  in `localStorage`, with JSON export/import and a shareable link for backup.
- **Boring and cheap** — GitHub Pages + one Cloudflare Worker. Zero monthly cost,
  no infrastructure to patch.

## Status

`seraph sync` reads Atlassian Statuspage documents, status.io documents,
service-specific JSON endpoints, and RSS feeds concurrently, retries transient
failures, and fails soft per service. It merges the previous snapshot so the
dashboard can show a 24-hour availability ranking. The catalog has 50 services
across cloud, AI, social, streaming, gaming, ecommerce, payments, and messaging,
including AWS and Google Cloud. `probe` sources still render as
`unknown` with an error note until that collector lands.

## Privacy and data retention

Favorites and named collections are the only user data. They stay in the
browser under `seraph.favorites.v1` and `seraph.collections.v1`; there is no
account, analytics, tracking, or server-side copy. Export backs up the active
collection, and “Copy link” creates a URL containing only its public service
slugs. Clearing site data, using private mode, or changing browser/device
removes the local copy — import each backup or open the link to restore it.
Account-based sync is intentionally left for a later, opt-in paid phase.

## How it works

```
Cloudflare cron (*/5) ──▶ GitHub Actions ──▶ seraph sync ──▶ site/static/data/state.json
                                                                  │
                                                           GitHub Pages
                                                                  │
                                                     browser + localStorage
```

1. A Cloudflare Worker dispatches the GitHub Actions workflow every five
   minutes. GitHub's own scheduler is best-effort and sleeps after inactivity;
   the Worker is not.
2. The CLI validates the YAML catalog, collects each service, and writes an
   atomic snapshot.
3. GitHub Pages serves the static site; the browser merges the snapshot with
   your local favorites and language choice.

More detail: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Quickstart

Requirements: Docker with Compose v2 and `make`. Go 1.27+ is only needed to run
the CLI outside Docker.

### Full local stack

```bash
git clone https://github.com/fabianoflorentino/seraph.git
cd seraph

make dev/up     # Hugo with live reload + collector loop
# open http://localhost:1314
```

The site reloads as you edit, and a container refreshes the snapshot every
minute. `make dev/logs` follows both services; `make dev/down` stops them. See
[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for ports, overrides, and
troubleshooting.

### CLI only

```bash
make validate   # check the catalog, no network
make catalog-inspect CATALOG_TARGET=example.com
make catalog-audit
make sync       # write site/static/data/state.json
make preview    # build and serve in the background on http://localhost:8000
make preview/logs   # follow requests and errors
make preview/stop   # stop the preview server
```

`make preview` returns immediately. Logs go to `.preview/server.log`; override the
port with `make preview PREVIEW_PORT=8080`.

Prefer plain Go for the CLI?

```bash
go run ./cmd/seraph validate --catalog catalog/packs
go run ./cmd/seraph sync --catalog catalog/packs
make site-build
python3 -m http.server -d site/public 8000
```

## CLI

| Command           | Purpose                                             |
| ----------------- | --------------------------------------------------- |
| `version`         | Print version, commit, and build date.              |
| `catalog add`     | Add a service to a pack and validate the catalog.   |
| `catalog inspect` | Discover status sources for a domain and draft YAML. |
| `catalog audit`   | Collect every source and report health.             |
| `validate`        | Validate every catalog pack; no network access.     |
| `sync`            | Collect services and write the snapshot.            |

Global flags: `--config <path>`, `--log-level <level>`. `sync` and `validate`
accept `--catalog <dir>` (default `catalog/packs`).

```bash
seraph --help
seraph catalog inspect github.com
seraph catalog audit --catalog catalog/packs
seraph validate --catalog catalog/packs
seraph sync --catalog catalog/packs --log-level debug
```

The inspect command probes known statuspage, status.io, JSON, and RSS shapes and
prints a draft entry. The audit command collects the full catalog and reports
latency, status, source type, and failures without writing a snapshot.

## Configuration

`config/seraph.yml` is optional; defaults apply when it is missing. Precedence
is **flag > environment > file > default**.

| Key           | Env                  | Default     |
| ------------- | -------------------- | ----------- |
| `out_dir`     | `SERAPH_OUT_DIR`     | `site/static/data` |
| `base_url`    | `SERAPH_BASE_URL`    | empty       |
| `timeout`     | `SERAPH_TIMEOUT`     | `8s`        |
| `concurrency` | `SERAPH_CONCURRENCY` | `12`        |
| `retries`     | `SERAPH_RETRIES`     | `2`         |
| `log_level`   | `SERAPH_LOG_LEVEL`   | `info`      |

See [docs/OPERATIONS.md](docs/OPERATIONS.md) for all constraints and runtime
notes.

## Catalog

Services are YAML, reviewed like code:

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

Packs live in `catalog/packs/`. Add a service without editing YAML by hand:

```bash
make catalog-add CATALOG_SLUG=example CATALOG_NAME=Example CATALOG_CATEGORY=cloud \
  CATALOG_OFFICIAL_URL=https://example.com CATALOG_STATUS_URL=https://status.example.com \
  CATALOG_SOURCE_TYPE=statuspage CATALOG_SOURCE_URL=https://status.example.com/api/v2/summary.json
```

The command validates every pack before writing and leaves the file untouched
on error. The schema recognizes `statuspage`, `statusio`, `rss`, `json`, and
`probe`; `statuspage`, `statusio`, `rss`, and `json` are implemented today.
Regions are
`global` plus ISO 3166-1 alpha-2 codes. The full schema, validation rules, and a
service-adding checklist are in [docs/CATALOG.md](docs/CATALOG.md).

## Deployment

1. Create the public repository and enable Pages with source *GitHub Actions*.
2. Deploy the dispatcher in `cloudflare/` with `GITHUB_TOKEN` (Actions: write)
   and `TRIGGER_SECRET` secrets.
3. Push to `main`; the workflow builds, validates, collects, and deploys.

Step-by-step instructions, monitoring, and troubleshooting:
[docs/OPERATIONS.md](docs/OPERATIONS.md).

### Container image

`Dockerfile` builds the site with Hugo, embeds the `seraph` binary, and serves
everything from an unprivileged nginx on port `8080`. The collector loop runs
inside the same container and writes to `/usr/share/nginx/html/data`.

```bash
make docker-build                 # fabianoflorentino/seraph:dev
make docker-run                   # http://localhost:8080
docker run --rm -p 8080:8080 \
  -v seraph-data:/usr/share/nginx/html/data \
  fabianoflorentino/seraph:dev
```

`image.yml` publishes `edge` from `main` and semver tags from `v*` tags to
Docker Hub; `release.yml` creates the GitHub release with Linux binaries.
Configure the `DOCKERHUB` environment secrets `DOCKERHUB_USERNAME` and
`DOCKERHUB_TOKEN` (Read & Write). If they are absent the image job warns and
skips publishing instead of failing CI.

The internal deployment lives in the
[srectl](https://github.com/fabianoflorentino/srectl) GitOps tree at
`deploy/projects/seraph`, exposed only inside the cluster as `seraph.localhost`.

## Development

```bash
make check      # fmt, vet, lint, race tests, coverage
make test       # race + shuffle
make lint       # golangci-lint
lefthook install
```

- Local stack: `make dev/up`, `dev/logs`, `dev/down`, `dev/rebuild`,
  `dev/collect`, `dev/reset`; see [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).
- The site is a Hugo project under `site/`; `make site-build` renders
  `site/public` with the same Hugo image used in CI.
- Coverage floor: **80%** (enforced by lefthook and CI).
- Code, identifiers, comments, and technical docs are in **English**; the site
  and user docs ship in English and Brazilian Portuguese.
- Project conventions for contributors and AI agents: [AGENTS.md](AGENTS.md).

## Roadmap

- **M1 (done)** — `statuspage` collector, retries, and per-service error handling.
- **M2** — first Brazilian service, exercising regions end to end.
- **M3** — `probe` collector for official health endpoints.
- **M4 (done)** — catalog at 50 services, Brazil + global.
- **History (done)** — 24-hour retention and availability ranking.
- **PWA install (done)** — manifest, icons, service worker, and offline shell.
- **Later** — PWA notifications and per-region latency comparisons.

## License

MIT. See [LICENSE](LICENSE).
