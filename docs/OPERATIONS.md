# Operations

English | [Português](OPERATIONS.pt-br.md)

## One-time setup

1. **Repository:** create the public repository `fabianoflorentino/seraph`.
2. **Pages:** Settings → Pages → Source: *GitHub Actions*.
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

## Local commands

```bash
make build       # bin/seraph with version metadata
make validate    # catalog validation only
make sync        # write site/data/state.json
make preview     # http://localhost:8000
make test        # race + shuffle
make coverage    # total coverage
make check       # fmt, vet, lint, test, coverage
```

Without `make`:

```bash
go run ./cmd/seraph validate --catalog catalog/packs
go run ./cmd/seraph sync --catalog catalog/packs
python3 -m http.server -d site 8000
```

## Configuration

`config/seraph.yml` is optional; built-in defaults apply when it is absent.
Precedence is **flag > environment > file > default**.

| Key           | Env                  | Default     | Notes                          |
| ------------- | -------------------- | ----------- | ------------------------------ |
| `out_dir`     | `SERAPH_OUT_DIR`     | `site/data` | Directory for `state.json`.    |
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
  previous deployment live.
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
