# seraph

> protects that which matters most

[![CI](https://github.com/fabianoflorentino/seraph/actions/workflows/ci.yml/badge.svg)](https://github.com/fabianoflorentino/seraph/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**English** | [Português](README.pt-br.md)

seraph is an independent, ad-free status board for the services you actually
depend on: banks, payment networks, carriers, clouds, messaging apps, and more.
It reads **official status pages and direct HTTP probes**, then publishes a
static JSON snapshot that a lightweight dashboard renders in English and
Brazilian Portuguese.

No account, no server, no database. Favorites are unlimited and live in your
browser.

## Why another status board?

Aggregators like Downdetector rely on user reports and page scraping: results
lag, ads get in the way, and there is no way to pin exactly the services you
care about. seraph inverts that:

- **Official signals first** — status pages and probes, never scraped
  aggregators.
- **Real freshness** — a reliable five-minute cron publishes a snapshot every
  5–20 minutes.
- **Yours** — unlimited favorites per region, stored only in `localStorage`.
- **Boring and cheap** — GitHub Pages + one Cloudflare Worker. Zero monthly cost,
  no infrastructure to patch.

## Status

Milestone **M0 (skeleton)** is complete: CLI, catalog validation, snapshot
schema, bilingual site, Cloudflare dispatcher, CI, and docs. Collectors are the
next milestone; until they land, `seraph sync` marks every service as `unknown`
so the full pipeline can be exercised end to end.

## How it works

```
Cloudflare cron (*/5) ──▶ GitHub Actions ──▶ seraph sync ──▶ site/data/state.json
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

Requirements: Go 1.27+, `make`, and Python 3 (for the local preview server).

```bash
git clone https://github.com/fabianoflorentino/seraph.git
cd seraph

make validate   # check the catalog, no network
make sync       # write site/data/state.json
make preview    # http://localhost:8000
```

Prefer plain Go?

```bash
go run ./cmd/seraph validate --catalog catalog/packs
go run ./cmd/seraph sync --catalog catalog/packs
python3 -m http.server -d site 8000
```

## CLI

| Command    | Purpose                                         |
| ---------- | ----------------------------------------------- |
| `version`  | Print version, commit, and build date.          |
| `validate` | Validate every catalog pack; no network access. |
| `sync`     | Collect services and write the snapshot.        |

Global flags: `--config <path>`, `--log-level <level>`. `sync` and `validate`
accept `--catalog <dir>` (default `catalog/packs`).

```bash
seraph --help
seraph validate --catalog catalog/packs
seraph sync --catalog catalog/packs --log-level debug
```

## Configuration

`config/seraph.yml` is optional; defaults apply when it is missing. Precedence
is **flag > environment > file > default**.

| Key           | Env                  | Default     |
| ------------- | -------------------- | ----------- |
| `out_dir`     | `SERAPH_OUT_DIR`     | `site/data` |
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
    regions:
      global: primary
    source:
      type: statuspage
      url: https://www.githubstatus.com/api/v2/summary.json
```

Packs live in `catalog/packs/`. Source types are `statuspage`, `statusio`,
`rss`, and `probe`; regions are `global` plus ISO 3166-1 alpha-2 codes. The full
schema, validation rules, and a service-adding checklist are in
[docs/CATALOG.md](docs/CATALOG.md).

## Deployment

1. Create the public repository and enable Pages with source *GitHub Actions*.
2. Deploy the dispatcher in `cloudflare/` with `GITHUB_TOKEN` (Actions: write)
   and `TRIGGER_SECRET` secrets.
3. Push to `main`; the workflow builds, validates, collects, and deploys.

Step-by-step instructions, monitoring, and troubleshooting:
[docs/OPERATIONS.md](docs/OPERATIONS.md).

## Development

```bash
make check      # fmt, vet, lint, race tests, coverage
make test       # race + shuffle
make lint       # golangci-lint
lefthook install
```

- Coverage floor: **80%** (enforced by lefthook and CI).
- Code, identifiers, comments, and technical docs are in **English**; the site
  and user docs ship in English and Brazilian Portuguese.
- Project conventions for contributors and AI agents: [AGENTS.md](AGENTS.md).

## Roadmap

- **M1** — `statuspage` collector and per-service error handling.
- **M2** — first Brazilian service, exercising regions end to end.
- **M3** — `probe` collector for services without a status page.
- **M4** — full catalog (~50 services, Brazil + global).
- **Later** — history retention, installable PWA with notifications.

## License

MIT. See [LICENSE](LICENSE).
