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
- **Official signals first.** Status pages and direct HTTP probes only.
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
                                                │ site/data/state.json
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
| Collectors       | planned (`internal/collector`) | Status pages and probes (roadmap).             |
| Site             | `site/`                      | Static dashboard, i18n, favorites.               |
| Dispatcher       | `cloudflare/`                | Reliable five-minute cron → `workflow_dispatch`. |
| CI/CD            | `.github/workflows/`          | Lint, tests, coverage, build, Pages deploy.      |

## Data flow

1. The Cloudflare Worker fires every five minutes and calls the GitHub API to
   dispatch `dashboard.yml` (also runnable manually or on push).
2. The workflow builds the CLI, validates the catalog, and runs `seraph sync`.
3. `sync` loads every pack under `catalog/packs`, resolves each service's
   source, collects observations, and builds a snapshot.
4. The snapshot is written atomically to `site/data/state.json`.
5. The whole `site/` directory is uploaded as the Pages artifact and deployed.
6. The browser fetches `data/state.json` with a cache-busting query, merges it
   with local favorites, and renders the board in English or Portuguese.

## Package map

| Path                 | Purpose                                                      |
| -------------------- | ------------------------------------------------------------ |
| `cmd/seraph`         | Entry point; prints errors and sets the exit code.           |
| `internal/cmd`       | `version`, `sync`, `validate`; shared config/logger helpers. |
| `internal/config`    | Typed configuration with validation and env overrides.       |
| `internal/catalog`   | Catalog types, validation rules, pack loading.               |
| `internal/state`     | Snapshot types, atomic persistence, schema version guard.    |
| `internal/version`   | Build metadata injected with `-ldflags`.                     |
| `catalog/packs`      | YAML service definitions grouped by audience/region.         |
| `config`             | Default `seraph.yml`.                                        |
| `site`               | Static dashboard (no build step).                            |
| `cloudflare`         | Cron dispatcher Worker.                                      |

Dependencies are injected through constructors; there is no package-level
mutable state. Collectors will accept an `*http.Client` and a clock so tests can
run without the network.

## Snapshot contract

`schema_version` is independent from the application version. Breaking changes
bump it; readers reject unknown versions with `ErrUnsupportedSchema`. Each
service carries status, region membership, observation time, optional latency,
optional error, and a short history series. The dashboard treats every field as
optional except `slug`, `name`, `category`, `regions`, and `status`.

Favorites are deliberately **not** in the snapshot. They live in
`localStorage` under `seraph.*.v1` keys, so the published artifact stays
anonymous and cacheable.

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

1. `internal/collector`: statuspage, status.io, RSS, and probe implementations.
2. History retention across runs (merge previous `state.json`).
3. PWA install with local notifications for favorites.
4. Per-region latency comparisons (multiple probe locations).
