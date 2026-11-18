# Local development

English | [Português](DEVELOPMENT.pt-br.md)

The local stack runs the Hugo site with live reload and a collector loop that
keeps writing fresh snapshots, so you see the dashboard behave like production
without deploying anything.

## Requirements

- Docker Engine 24+ with Compose v2
- `make`
- Go 1.27+ only if you want to run the CLI outside Docker

The Hugo image is pinned in `docker-compose.yml`, so Hugo does not need to be
installed on the host.

## Quick start

```bash
make dev/up      # build and start Hugo + collector
make dev/logs    # follow both services
```

Open <http://localhost:1314>. The collector validates the catalog, writes
`site/static/data/state.json`, and repeats every 60 seconds. Hugo serves the
snapshot at `/data/state.json` with `Cache-Control: no-store`.

Stop everything with `make dev/down`.

## What runs

| Service     | Image                        | Port        | Role                                                        |
| ----------- | ---------------------------- | ----------- | ----------------------------------------------------------- |
| `site`      | `hugomods/hugo:0.158.0`      | `1314→1313` | `hugo server`, live reload, mounts `./site`.                |
| `collector` | built from `Dockerfile.dev`  | none        | Loops `seraph validate` + `seraph sync` every 60 seconds.   |

Host port `1314` avoids the usual Hugo port `1313`, which is often taken by
another project. Override it when needed:

```bash
make dev/up SITE_PORT=1414
make dev/logs
```

The interval is configurable too:

```bash
make dev/up DEV_INTERVAL=15
```

The Makefile passes your `UID`/`GID` to both containers, so files created in
`site/static/data` and `.hugo_build.lock` stay owned by your user.

## Editing the site

| Path                            | Purpose                                       |
| ------------------------------- | --------------------------------------------- |
| `site/hugo.toml`                | Hugo config, cache headers for `/data/**`.    |
| `site/layouts/index.html`       | Dashboard markup (Hugo home template).        |
| `site/static/style.css`         | Styles.                                       |
| `site/static/app.js`            | Fetch, filters, favorites, i18n wiring.       |
| `site/static/i18n/en.js`        | English strings.                              |
| `site/static/i18n/pt-br.js`     | Brazilian Portuguese strings.                 |
| `site/static/data/state.json`   | Generated snapshot, ignored by git.           |

HTML, CSS, and JS changes reload the browser automatically. Keep English and
Portuguese strings in sync when adding UI text.

## Editing the collector

The collector binary is built into the image. After Go changes:

```bash
make dev/rebuild     # rebuild the collector image and restart the service
```

Catalog and configuration are bind-mounted read-only, so adding a service or
changing `config/seraph.yml` takes effect on the next cycle without a rebuild:

```bash
make dev/collect     # run one cycle immediately
```

## Production-like builds

```bash
make site-build      # Hugo build into site/public
make preview         # build, then serve on http://localhost:8000
```

`make site-build` uses the same Hugo image as CI, so template errors surface
locally.

## Resetting

```bash
make dev/down
make dev/reset       # stop, remove containers, delete the generated snapshot
```

Hugo output (`site/public`, `site/resources`, `.hugo_build.lock`) and the
snapshot are all git-ignored.

## Troubleshooting

| Symptom                              | Fix                                                              |
| ------------------------------------ | ---------------------------------------------------------------- |
| `Bind for 0.0.0.0:1314 failed`       | Another process uses the port; `make dev/up SITE_PORT=1414`.     |
| Snapshot not changing                | `make dev/logs`; check catalog validation errors.                |
| `create output dir ... permission denied` | Use the Makefile targets so UID/GID are passed to Compose.  |
| Changes to Go code ignored           | `make dev/rebuild`.                                              |
| Hugo image pull fails                | Check network access, or pre-pull `hugomods/hugo:0.158.0`.        |
| Need the Worker locally              | `cd cloudflare && npx wrangler dev` (not part of Compose).        |

## Quality gate

Before committing:

```bash
make check
```

This runs formatting, vet, golangci-lint, race tests, and coverage. The
`pre-push` hook enforces the same gate plus `go mod verify`.
