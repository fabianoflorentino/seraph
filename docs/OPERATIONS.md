# Operations

English | [Português](OPERATIONS.pt-br.md)

## One-time setup

1. **Repository:** create the public repository `fabianoflorentino/seraph`.
2. **Pages:** Settings → Pages → Source: *GitHub Actions*. Set the custom
   domain `seraph.fabianoflorentino.dev` and point its DNS CNAME to
   `fabianoflorentino.github.io`. Hugo publishes `site/static/CNAME`.
3. **Workflow permissions:** Settings → Actions → General → allow GitHub
   Actions to create deployments (default for Pages).
4. **Cloudflare Worker:**
   ```bash
   cd cloudflare
   npx wrangler login
   npx wrangler secret put GITHUB_TOKEN    # fine-grained PAT, Actions: write
   npx wrangler secret put TRIGGER_SECRET  # random string
   npx wrangler deploy
   ```
   The PAT needs *Actions: write* on this repository only. No other scopes.
5. **Verify:**
   ```bash
   curl https://seraph-dispatch.<account>.workers.dev/health
   curl -X POST https://seraph-dispatch.<account>.workers.dev/trigger \
     -H "Authorization: Bearer $TRIGGER_SECRET"
   ```
   The second call should return `{"ok":true}` and start a run within seconds.

## Cadence and freshness

- Cloudflare cron fires every five minutes; `dashboard.yml` takes a couple of
  minutes, so published data is usually 5–20 minutes old.
- A GitHub `schedule` entry every 15 minutes is a fallback if the Worker fails.
- Pushing to `main` also refreshes the site, which keeps catalog edits visible
  immediately.
- Before collecting, the workflow restores the last published `state.json` from
  Pages. `sync` merges its points (up to 288 per service, about 24h at the
  five-minute cadence) so availability survives ephemeral runners. If the
  restore fails, the run starts history fresh.

## Local commands

```bash
make dev/up      # Hugo + collector at http://localhost:1314
make dev/logs    # follow both services
make dev/down    # stop the stack
make build       # bin/seraph with version metadata
make validate    # catalog validation only
make catalog-inspect CATALOG_TARGET=example.com
make catalog-audit
make sync        # write site/static/data/state.json
make preview     # build the Hugo site, serve http://localhost:8000
make test        # race + shuffle
make coverage    # total coverage
make check       # fmt, vet, lint, test, coverage
```

The full stack is documented in [DEVELOPMENT.md](DEVELOPMENT.md). Without `make`:

```bash
go run ./cmd/seraph validate --catalog catalog/packs
go run ./cmd/seraph sync --catalog catalog/packs
docker compose run --rm --no-deps site hugo --minify --gc
python3 -m http.server -d site/public 8000
```

## Container and internal deployment

The production image is defined by the root `Dockerfile`:

- Hugo builds `site/` with `HUGO_BASEURL` (default
  `http://seraph.localhost/`).
- Go builds a static `seraph` binary for the target platform.
- The runtime is `nginx-unprivileged` on port `8080`; `docker/entrypoint.sh`
  validates the catalog, starts a collector loop, and serves the site.
- The collector writes to `SERAPH_OUT_DIR` (default
  `/usr/share/nginx/html/data`). Mount a volume there to keep history across
  container restarts.

```bash
make docker-build
make docker-run                 # http://localhost:8080
make docker-push IMAGE_TAG=v0.1.0
```

| Setting | Default in image | Purpose |
| --- | --- | --- |
| `SERAPH_OUT_DIR` | `/usr/share/nginx/html/data` | Snapshot directory. |
| `SERAPH_BASE_URL` | unset | Optional public HTTPS snapshot URL. |
| `SERAPH_SYNC_INTERVAL` | `300` | Seconds between collector runs. |

Health endpoints: `/healthz` for the process, `/data/state.json` for data
readiness. The site is served at `/`.

### GitHub Container Registry / Docker Hub workflows

- `.github/workflows/image.yml` pushes `edge` on `main` and semver tags on
  `v*`; it needs the `DOCKERHUB` environment with `DOCKERHUB_USERNAME` and
  `DOCKERHUB_TOKEN` (Read & Write). Without them it warns and skips.
- `.github/workflows/release.yml` builds Linux amd64/arm64 archives,
  checksums, and the GitHub release for any `v*` tag.
- The srectl GitOps app watches `deploy/projects/seraph`; update the image tag
  there to roll out a new release.

## Configuration

`config/seraph.yml` is optional; built-in defaults apply when it is absent.
Precedence is **flag > environment > file > default**.

| Key           | Env                  | Default     | Notes                          |
| ------------- | -------------------- | ----------- | ------------------------------ |
| `out_dir`     | `SERAPH_OUT_DIR`     | `site/static/data` | Directory for `state.json`.    |
| `base_url`    | `SERAPH_BASE_URL`    | empty       | Optional HTTPS public URL.     |
| `timeout`     | `SERAPH_TIMEOUT`     | `8s`        | Per-request timeout, 1–60s.    |
| `concurrency` | `SERAPH_CONCURRENCY` | `12`        | Parallel collections, 1–128.   |
| `retries`     | `SERAPH_RETRIES`     | `2`         | Retries per source, 0–10.      |
| `log_level`   | `SERAPH_LOG_LEVEL`   | `info`      | `debug`, `info`, `warn`, `error`. |

`--config` selects another file; `--log-level` overrides the configured level.

## Monitoring

- Worker health: `GET /health` returns JSON with `"status":"ok"`.
- Dispatch failures appear in Cloudflare logs (`wrangler tail`).
- Workflow failures appear under Actions and fail the Pages deploy, leaving the
  previous deployment live. A failed Pages deploy retries with exponential
  backoff (1 min, doubling up to 1 h) before the job finally fails.
- The dashboard shows an error banner when the snapshot cannot be fetched; it
  keeps the last rendered data.

## Troubleshooting

| Symptom                              | Likely cause                                   | Fix                                                      |
| ------------------------------------ | ---------------------------------------------- | -------------------------------------------------------- |
| Snapshot never changes               | Worker cron not deployed or PAT expired         | `npx wrangler tail`, re-issue PAT, redeploy.             |
| `GitHub dispatch failed: 401/403`    | PAT missing *Actions: write*                    | Update the fine-grained token, `wrangler secret put`.    |
| `GitHub dispatch failed: 404`        | Wrong owner/repo/workflow name or ref           | Check `wrangler.toml` vars and the workflow filename.    |
| `unsupported snapshot schema version`| Site/CLI version mismatch                       | Deploy the CLI and site from the same commit.            |
| `invalid catalog: ... duplicate slug`| Same slug in two packs                          | Keep one definition, reference it from the right region. |
| Banner in the dashboard, old data    | Pages deploy failed                             | Inspect the latest Actions run; data stays stale, not lost. |
| Catalog valid but card missing       | Region or favorites filter                      | Check the region buttons and the favorites toggle.       |
| Card shows `unknown` with an error   | Source unreachable, HTTP error, or bad payload  | Open the status page, then check the workflow log for the service slug. |
| `dial tcp 0.0.0.0:443`               | DNS sinkhole/adblock or split DNS on the runner | Allowlist the status host or use its canonical Statuspage host.          |
| Availability panel missing           | Fewer than three observations in 24h            | Let the collector run; history forms after a few checks.                 |

## Incident playbook

1. Check the Worker: `/health` and `wrangler tail`.
2. Check Actions: is `dashboard.yml` dispatched? Does it fail at validate, sync,
   or deploy?
3. If GitHub is degraded, wait: the last Pages deployment keeps serving.
4. If a single service misbehaves, fix its catalog entry; other services are
   unaffected.
5. Record user-visible changes in `CHANGELOG.md`.

## Cost

- GitHub Actions and Pages: free for public repositories within standard limits.
- Cloudflare Workers: free plan covers 100k requests/day; the cron uses 288
  invocations/day.
- No database, no object storage, no paid monitoring.

## Upgrades

- Dependabot opens weekly PRs for Go modules, the Worker's `wrangler`, and
  Actions. CI gates them.
- Bump `schema_version` only for breaking snapshot changes; update the site and
  CLI in the same release.
