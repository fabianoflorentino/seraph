# Cloudflare dispatcher

A tiny Worker that triggers the GitHub Actions `dashboard.yml` workflow every
five minutes. GitHub's own cron is best-effort and disables schedules after 60
days of repository inactivity; Cloudflare cron is reliable, and a
`workflow_dispatch` run counts as activity.

## Endpoints

| Method | Path       | Purpose                                      |
| ------ | ---------- | -------------------------------------------- |
| GET    | `/health`  | Liveness probe.                              |
| POST   | `/trigger` | Manual dispatch; requires the bearer secret. |

## Setup

```bash
cd cloudflare
npx wrangler login
npx wrangler secret put GITHUB_TOKEN   # fine-grained PAT, Actions: write
npx wrangler secret put TRIGGER_SECRET # random string for POST /trigger
npx wrangler deploy
```

`GITHUB_TOKEN` must belong to an account that can dispatch workflows in
`fabianoflorentino/seraph`. `TRIGGER_SECRET` protects the manual endpoint:

```bash
curl -X POST https://seraph-dispatch.<account>.workers.dev/trigger \
  -H "Authorization: Bearer $TRIGGER_SECRET"
```

## Local development

```bash
npx wrangler dev
```

Add a `.dev.vars` file with `GITHUB_TOKEN` and `TRIGGER_SECRET` to test the
dispatch path locally.
