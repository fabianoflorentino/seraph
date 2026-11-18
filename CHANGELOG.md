# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Go module, Cobra CLI, and `version`, `sync`, and `validate` commands.
- YAML catalog schema with pack loading, validation, and duplicate detection.
- Snapshot schema with atomic writes and schema versioning.
- Statuspage collector with bounded concurrency, per-attempt timeouts, retries
  for transient failures, component scoping, and per-service error handling.
- status.io, service-specific JSON (Slack, Stripe, AWS, and GCP), and RSS
  collectors.
- Initial 24-service catalog covering cloud, AI, social, streaming, gaming,
  ecommerce, payments, and messaging, with an `ai` category.
- Clickable service cards that open the official site, layered component
  shadows, and a reduced-motion-aware ember backdrop.
- Animated All/Favorites segmented control and EN/pt-BR switch with smooth
  transitions, accessible state, and a reduced-motion fallback.
- Optional `status_url` on service cards, with `official_url` as fallback;
  GitHub now links to its status page.
- Collapsible category sections with persisted state, attention badges, and an
  overview with service/status/category counts.
- The Attention overview card filters non-operational services; the Services
  card clears the filter and Categories toggles collapse state.
- History retention across runs (up to 288 points per service) and a 24-hour
  availability ranking on the dashboard.
- `seraph catalog add` and `make catalog-add` for validated YAML catalog edits.
- Category and region tags on service cards; the region filter only appears
  when more than one region is available.
- Configuration file with `SERAPH_*` environment overrides.
- Static bilingual dashboard (English and Brazilian Portuguese), built with
  Hugo.
- Docker Compose development stack with Hugo live reload and a collector loop,
  plus `dev/*` and site targets in the Makefile.
- Cloudflare Worker dispatcher for scheduled GitHub Actions runs.
- CI pipeline with lint, tests, coverage gate, build, and vulnerability scan.
- Project tooling: Makefile, lefthook, golangci-lint, and Dependabot.

### Changed

- `make preview` now serves the built site in the background and returns the
  terminal, with `preview/logs`, `preview/status`, and `preview/stop` targets.
- The ember backdrop now spans the whole document instead of only the first
  viewport.

### Fixed

- The snapshot error banner stayed visible because its `display: flex` rule
  overrode the `hidden` attribute; `[hidden]` now always hides elements.

[Unreleased]: https://github.com/fabianoflorentino/seraph/commits/main
