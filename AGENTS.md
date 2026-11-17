# Project Conventions

These rules apply to all work in this repository. Extended Go guidance lives in
the agent skills listed below; load a skill when a task matches its description.

## Project Overview

seraph is a static status board: a Go collector reads a YAML catalog of
services, observes official status pages and probes, and publishes a JSON
snapshot consumed by a bilingual site on GitHub Pages. See
[`README.md`](README.md) and [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## 1. English in Code, Bilingual Site and User Docs

Keep code and engineering artifacts in English: identifiers, comments, commit
messages, technical documentation, configuration, and scripts.

The public surface is bilingual. The site ships English (default) and Brazilian
Portuguese via `site/i18n/en.js` and `site/i18n/pt-br.js`; user documentation
ships as `README.md` / `README.pt-br.md` and `docs/NAME.md` /
`docs/NAME.pt-br.md`. When one changes, update its pair in the same commit.

## 2. Skills to Load

Always start from the `golang-how-to` orchestrator when touching Go code. Then
load the skills that match the task:

- Core: `golang-code-style`, `golang-data-structures`,
  `golang-design-patterns`, `golang-documentation`, `golang-error-handling`,
  `golang-modernize`, `golang-naming`, `golang-safety`, `golang-security`,
  `golang-testing`, `golang-troubleshooting`.
- Task-specific: `golang-cli` (command tree, flags, exit codes),
  `golang-project-layout` (packages, layout), `golang-lint` (golangci-lint),
  `golang-continuous-integration` (workflows, Dependabot),
  `golang-rest` (future HTTP collectors), `golang-benchmark` (probe latency
  work).

Do not skip the orchestrator: it disambiguates overlapping skills.

## 3. Test Coverage ≥ 80% (Always)

Total coverage must stay at or above 80%. The `pre-push` hook and CI enforce it;
treat it as a floor. Never add production code without tests. Prefer table-driven
tests, `t.Parallel()` where safe, `fstest.MapFS` for catalog fixtures, and
`t.TempDir()` for state writes. Tests that mutate the environment must not run in
parallel and need an explaining `//nolint:paralleltest` directive.

## 4. Code Style

- Match the surrounding file: naming, comment density, structure.
- Comments explain **why**; do not restate the code.
- No dead code, no speculative abstractions, no half-built public surface.
- Small, scoped, behavior-preserving changes. Separate refactors from features.
- Constructors inject dependencies; no package-level mutable state.
- Return `%w`-wrapped errors with sentinel errors for callers.
- Use `slog` for logging; user-facing command output goes to the command's
  `OutOrStdout()`.

## 5. Cross-Cutting Invariants (Do Not Violate)

1. **Official signals only.** Never scrape Downdetector or similar aggregators.
   Use status pages, status APIs, incident feeds, or direct probes.
2. **Static snapshot, no database.** The only runtime artifact is
   `site/data/state.json`, written atomically. History arrives by merging the
   previous snapshot, never by adding storage.
3. **Favorites are client-side.** They stay in `localStorage` under versioned
   `seraph.*.v1` keys. Never send them to a server or add an account system.
4. **Schema discipline.** `schema_version` gates readers; breaking changes bump
   it and update CLI, site, and docs together.
5. **Catalog is data.** Adding a service is a YAML change that must pass
   `seraph validate`; never hard-code service lists in Go or JavaScript.
6. **HTTPS only.** Source and official URLs must be absolute HTTPS.
7. **No secrets in the repository.** The GitHub token and trigger secret live in
   Cloudflare; the workflow uses the default `GITHUB_TOKEN`.
8. **Fail soft per service.** A broken source yields `unknown` with an error
   note; it must never abort the whole collection run.
9. **Scheduling is external.** Cloudflare cron triggers `workflow_dispatch`;
   GitHub `schedule` is only a fallback.

## 6. Validation Gate

Every change must pass the hooks:

- `pre-commit`: golangci-lint + `go vet`
- `pre-push`: `go mod verify`, golangci-lint, `go vet`, race tests, coverage ≥ 80%

Run the full local gate before declaring work ready:

```bash
make check
lefthook run pre-push
```

## 7. Toolchain Notes

- Go 1.27.1, managed with `asdf` (`.tool-versions`). `golangci-lint` comes from
  the asdf Go `GOBIN` with shims in `~/.asdf/shims`; hooks prefix `PATH`
  accordingly.
- `golangci-lint` must be built with a toolchain compatible with the project Go
  version. If lint reports impossible errors, check `golangci-lint version`.
- Run `govulncheck` when adding or bumping dependencies.
- After changing dependencies, run `go mod tidy` and `go mod verify`.

## 8. Documentation Map

- `README.md` / `README.pt-br.md` — features, quickstart, CLI, config.
- `docs/ARCHITECTURE.md` — components, data flow, snapshot contract.
- `docs/CATALOG.md` — schema, enums, sources, validation, adding services.
- `docs/OPERATIONS.md` — deployment, scheduling, monitoring, troubleshooting.
- `CHANGELOG.md` — user-visible changes (Keep a Changelog).
- `cloudflare/README.md` — Worker deployment and secrets.

## 9. Git Workflow

- Commit small, thematic, English messages. Never commit or push unless
  explicitly asked.
- Keep the generated snapshot out of commits (`site/data/*.json` is ignored); CI
  publishes it.
- Update `CHANGELOG.md` for user-visible behavior.
