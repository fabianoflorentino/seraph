# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Production container image: multi-stage Hugo + Go build, unprivileged nginx
  runtime, in-container collector loop, `/healthz`, and Docker/Make targets.
- Docker Hub image workflow for `edge` and `v*` tags, plus a tag-driven GitHub
  release workflow with Linux amd64/arm64 archives and checksums.
- Go module, Cobra CLI, and `version`, `sync`, and `validate` commands.
- YAML catalog schema with pack loading, validation, and duplicate detection.
- Snapshot schema with atomic writes and schema versioning.
- Statuspage collector with bounded concurrency, per-attempt timeouts, retries
  for transient failures, component scoping, and per-service error handling.
- status.io, service-specific JSON (Slack, Stripe, AWS, and GCP), and RSS
  collectors.
- Initial 50-service catalog covering cloud, AI, social, streaming, gaming,
  ecommerce, payments, and messaging, with an `ai` category.
- Catalog discovery (`catalog inspect`) with draft YAML generation and a
  read-only source health audit (`catalog audit`).
- Clickable service cards that open the official site, layered component
  shadows, and a reduced-motion-aware ember backdrop.
- Animated All/Favorites segmented control and EN/pt-BR switch with smooth
  transitions, accessible state, and a reduced-motion fallback.
- Optional `status_url` on service cards, with `official_url` as fallback;
  GitHub now links to its status page.
- Collapsible category sections with persisted state, attention badges, and an
  overview with service/status/category counts.
- The Operational, Attention, and Outage overview cards filter services; the
  Services card clears the filter and Categories toggles collapse state.
  Filter changes use a reduced-motion-aware view transition.
- Relative update age with a stale indicator at 20 minutes, a manual snapshot
  refresh button, and a direct link to the Dashboard workflow.
- History retention across runs (up to 288 points per service) and a 24-hour
  availability ranking on the dashboard.
- Installable PWA shell with manifest, icons, service worker, and offline
  fallback for assets and the latest snapshot.
- Local-only favorite backup/share controls: JSON export/import and a link
  containing only public service slugs, plus an explicit privacy notice.
- Named favorite collections with local switching, creation, rename, delete,
  and collection-scoped export/share.
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

- The container build now targets the internal `seraph.localhost` origin; no
  public DNS name is required.
- `make preview` now serves the built site in the background and returns the
  terminal, with `preview/logs`, `preview/status`, and `preview/stop` targets.
- The ember backdrop now spans the whole document instead of only the first
  viewport.

### Fixed

- The snapshot error banner stayed visible because its `display: flex` rule
  overrode the `hidden` attribute; `[hidden]` now always hides elements.
- Segment now uses its canonical Statuspage API host so a local DNS sinkhole
  returning `0.0.0.0` cannot produce a false `unknown`.
- Category attention badges now separate major (red), degraded (amber), and
  unknown (gray) counts instead of one aggregate amber badge.

[Unreleased]: https://github.com/fabianoflorentino/seraph/commits/main
