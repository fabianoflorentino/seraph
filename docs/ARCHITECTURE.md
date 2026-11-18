# Architecture

English | [Português](ARCHITECTURE.pt-br.md)

## Goals

seraph is a cheap, dependable status board. It answers one question — "is the
service I depend on up right now?" — using official sources instead of scraping
aggregators, and it costs nothing to operate.

Design constraints:

- **Zero infrastructure.** Static JSON on GitHub Pages, no server or database.
- **Real freshness.** A snapshot every 5–20 minutes, driven by a reliable cron.
- **Unlimited favorites.** Stored in the browser; no accounts, no tracking.
- **Official signals first.** Official status pages and feeds only.
- **Boring operations.** If GitHub is down, the last snapshot keeps serving.

## Components

```
┌─────────────────────────┐        ┌──────────────────────────┐
│ Cloudflare Worker cron  │──────▶ │ GitHub Actions           │
│ seraph-dispatch */5 * * │       │ workflow_dispatch        │
└─────────────────────────┘        └────────────┬─────────────┘
                                                │
                 ┌──────────────────────────────▼────────────────┐
                 │ seraph CLI                                    │
                 │ catalog.Load → collect → state.WriteAtomic    │
                 └──────────────────────────────┬────────────────┘
                                                │ site/static/data/state.json
                                                ▼
                                      ┌───────────────────┐
                                      │ GitHub Pages      │
                                      │ static dashboard  │
                                      └───────────────────┘
```

| Component        | Location                     | Responsibility                                   |
| ---------------- | ---------------------------- | ------------------------------------------------ |
| CLI              | `cmd/seraph`, `internal/cmd` | Command tree, config resolution, logging.        |
| Catalog          | `internal/catalog`           | Load, validate, and merge YAML packs.            |
| State            | `internal/state`             | Snapshot schema, atomic JSON writes, reads.      |
| Config           | `internal/config`            | File + `SERAPH_*` environment + flag precedence. |
| Collectors       | `internal/collector`         | Statuspage, status.io, JSON, and RSS observations. |
| Site             | `site/`                      | Hugo site: dashboard, i18n, favorites.           |
| Dispatcher       | `cloudflare/`                | Reliable five-minute cron → `workflow_dispatch`. |
| CI/CD            | `.github/workflows/`          | Lint, tests, coverage, build, Pages deploy.      |

## Data flow

1. The Cloudflare Worker fires every five minutes and calls the GitHub API to
   dispatch `dashboard.yml` (also runnable manually or on push).
2. The workflow builds the CLI, validates the catalog, and runs `seraph sync`.
3. `sync` loads every pack under `catalog/packs` and `internal/collector`
   observes each service concurrently with bounded concurrency, per-attempt
   timeouts, and retries for transient failures.
4. The snapshot is written atomically to `site/static/data/state.json`.
5. Hugo renders `site/` into `site/public`, which is uploaded as the Pages
   artifact and deployed.
6. The browser fetches `data/state.json` with a cache-busting query, merges it
   with local favorites, and renders the board in English or Portuguese.

## Package map

| Path                 | Purpose                                                      |
| -------------------- | ------------------------------------------------------------ |
| `cmd/seraph`         | Entry point; prints errors and sets the exit code.           |
| `internal/cmd`       | `version`, `sync`, `validate`; shared config/logger helpers. |
| `internal/config`    | Typed configuration with validation and env overrides.       |
| `internal/catalog`   | Catalog types, validation rules, pack loading.               |
| `internal/collector` | Statuspage client, retries, concurrency, status mapping.     |
| `internal/state`     | Snapshot types, atomic persistence, schema version guard.    |
| `internal/version`   | Build metadata injected with `-ldflags`.                     |
| `catalog/packs`      | YAML service definitions grouped by audience/region.         |
| `config`             | Default `seraph.yml`.                                        |
| `site`               | Hugo site: layouts, static assets, i18n, snapshot.            |
| `cloudflare`         | Cron dispatcher Worker.                                      |

Dependencies are injected through constructors; there is no package-level
mutable state. Collectors accept an `*http.Client` and a clock so tests run
without the network.

## Local development

`docker-compose.yml` runs two services. `site` starts the pinned Hugo image in
server mode on <http://localhost:1314> with live reload, mounting `site/`.
`collector` builds `Dockerfile.dev`, then validates and syncs every minute into
the mounted `site/static/data`, so the dashboard shows the full loop without
deploying. Details and overrides: [DEVELOPMENT.md](DEVELOPMENT.md).

## Snapshot contract

`schema_version` is independent from the application version. Breaking changes
bump it; readers reject unknown versions with `ErrUnsupportedSchema`. Each
service carries status, region membership, observation time, optional latency,
optional error, optional `official_url` and `status_url`, and a history series.
The CLI merges the previous snapshot, caps each series at 288 points (about 24h
at five-minute cadence), and the dashboard computes a 24-hour availability
ranking from those points. The dashboard links cards to `status_url` when
present, falling back to `official_url`. It treats every field as optional
except `slug`, `name`, `category`, `regions`, and `status`.

Favorites and named collections are deliberately **not** in the snapshot. They
live in `localStorage` under `seraph.*.v1` keys, so the published artifact stays
anonymous and cacheable. This is the privacy boundary: there is no account,
server copy, or analytics. Users can export the active collection as a
versioned JSON backup or copy a link that contains only service slugs; clearing
browser data or changing devices requires one of those to restore. Account sync
remains a later opt-in feature.

## Reliability choices

- **Atomic writes:** write to a temp file in the target directory, then rename,
  so readers never observe a partial snapshot.
- **Fail soft per service:** one broken source never blocks the whole run; it
  becomes `unknown` with an error note.
- **Catalog is data:** adding a service is a YAML change reviewed like code.
- **Validation before collection:** schema errors fail the run early, with the
  file and field in the message.
- **No secrets in the repo:** the GitHub token and trigger secret live in
  Cloudflare; the workflow uses the default `GITHUB_TOKEN`.

## Roadmap

1. `internal/collector`: probe implementation for official health endpoints.
2. PWA notifications for favorites (installability is implemented).
3. Per-region latency comparisons (multiple probe locations).
4. Expand the verified catalog and region coverage.
