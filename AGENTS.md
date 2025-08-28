# Repository Guidelines

## Project Structure & Module Organization
- Root module: `github.com/k0sproject/k0sctl` (Go, aiming to keep version up to date).
- Source layout:
  - `main.go`: entrypoint for the CLI.
  - `cmd/`: CLI commands/subcommands (e.g., `apply.go`, `init.go` which map to commands like "k0sctl apply", "k0sctl init").
  - `phase/`: orchestration phases for cluster operations and the phase manager that runs them.
  - `pkg/`: reusable packages (e.g., `k0sfeature/`, `manifest/`).
  - `internal/`: internal helpers (e.g., `shell/`).
  - `smoke-test/`: end‑to‑end smoke tests (Make targets, uses github.com/k0sproject/bootloose for test machines).
  - `examples/`: somewhat outdated examples for Terraform use.

## What The App Does
- Bootstraps and manages k0s Kubernetes clusters by connecting to nodes over SSH using github.com/k0sproject/rig as the SSH driver and target OS/distro compatibility layer.
- Core operations: "apply" connects to hosts, installs k0s, configures, and starts it; "reset" uninstalls k0s and cleans up; "init" creates a sample config, "kubeconfig" generates a config to access the cluster using kubectl.
- Input: `k0sctl.yaml` describing hosts, roles, and k0s version/config; output: actions executed remotely with clear phase logs.
- `urfave/cli` is used as the CLI framework.

## Build, Test, and Development Commands
- Build local binary: `make k0sctl` (outputs `./k0sctl`).
- Cross‑builds: `make build-all` (artifacts in `bin/`).
- Run unit tests: `make test` or `go test -v ./...`.
- Lint: `make lint` (uses golangci-lint defaults).
- Smoke tests (CI matrix in `.github/workflows/smoke.yml`): run locally with targets like `make smoke-basic`, `make smoke-upgrade`. Requires a working `bootloose` (k0sproject/bootloose) setup and virtualization/container tooling; commonly set `LINUX_IMAGE` env.
- Run locally: `go run .` or `./k0sctl --help` after build.

## Coding Style & Naming Conventions
- Follow standard Go style (`gofmt`, `goimports`), tabs for indentation.
- Package names: short, lowercase; files use `snake_case.go`; tests `*_test.go`.
- Keep CLI flags/env vars consistent with `cmd/` patterns.
- Use latest Go or at least the version in `go.mod` (toolchain pinned).

## Testing Guidelines
- Unit tests: colocate `*_test.go`; use Go `testing`/`testify` as needed.
- Coverage: no minimum enforced; run `go test -cover ./...` locally.
- Smoke tests: mirror CI matrix in `.github/workflows/smoke.yml` (e.g., `smoke-basic`, `smoke-upgrade`, `smoke-reset`, `smoke-backup-restore`). Run locally if you have `bootloose` working.
- Name tests after behavior (e.g., `TestValidateHosts_*`).
 - Transport in tests: prefer the `ssh` transport (Go `crypto/ssh`) for end-to-end/smoke coverage; only use `openSSH` when explicitly testing that transport mode.
 - Unit tests must not connect to real hosts or change remote state. Prefer `localhost` connections (Rig’s localhost driver) or mocks/fakes for execution and file transfer.
 - Keep tests hermetic and deterministic; avoid relying on external network resources, time-based flakiness, or host-specific configuration.

## Commit & Pull Request Guidelines
- Commits: concise, imperative; keep history clean by rebasing/editing instead of piling “fix typo” commits. Multiple well‑maintained commits welcome but maintainers often squash on merge.
  - Example: `Fix panic when parsing multi-doc YAML`.
- PRs: include problem statement, approach, impact; link issues; include sample config/output when changing CLI or phases.
- CI: All checks must pass (lint, unit, smoke). Update `README.md` (source of truth) when config fields or behavior change; update `examples/` if applicable.

## Security & Compliance
- DCO required: sign off commits (`git commit -s`) with `Signed-off-by: Name <email>`.
- Examples and tests must only use private addresses and redacted or test-generated key files.
- Prefer the `openSSH` transport or SSH agent usage over embedding private keys in configs.

## Architecture Overview
- CLI in `cmd/` maps subcommands to actions in action/ which compose phases and hand them to `phase.Manager`.
- Config flow: parse `k0sctl.yaml` → build `v1beta1.Cluster` → defaults via `creasty/defaults` → run phases with logging.
- Dry‑run: `Manager.DryRun=true` records intended actions (`DryMsg`, `Wet`) and prints a per‑host plan.

## Phase Manager
- Contract: `type Phase interface { Run(ctx) error; Title() string }`.
- Optional interfaces used by the manager:
  - `withconfig.Prepare(*Cluster) error` for precompute/validation.
  - `conditional.ShouldRun() bool` to skip dynamically.
  - `beforehook.Before(title) error` and `afterhook.After(err) error` for hooks.
  - `withDryRun.DryRun() error` for alternate dry‑run behavior.
  - `withcleanup.CleanUp()` on failure paths; `withmanager.SetManager(*Manager)` for access to helpers.
- Concurrency: `Manager.Concurrency` and `ConcurrentUploads` are respected by phases that parallelize work.
- Naming: phase titles must be human‑readable and describe the intention (e.g., `PrepareHosts`, `InstallWorkers`, `UpgradeWorkers`). Files use `snake_case.go` matching the title.

## Transport Layer: k0sproject/rig
- Rig provides connection/execution primitives used by phases: SSH (native) and OpenSSH client modes, file transfers, sudo elevation, bastion support, env propagation, and structured logging.
- Windows/WinRM exists in rig but k0sctl targets Linux nodes; SSH is the primary transport here. Windows support may be added in the future.
- Repo: `github.com/k0sproject/rig` (use v0.x). The `main` branch is for the future v2 API which k0sctl will eventually migrate to.
 - YAML keys: `ssh:` selects the native Go `crypto/ssh` transport (preferred default), `openSSH:` uses the locally installed OpenSSH client, and `localhost:` uses the local machine as the target.

## Extending & Adding Phases
- Implement a new `phase.Phase` in `phase/` with a clear `Title()` and idempotent `Run(ctx)`; use optional interfaces where appropriate (Prepare, DryRun, hooks).
- Wire it from the relevant action in `cmd/` by calling `manager.AddPhase(...)` in correct order.
- Use manager helpers for dry‑run (`Wet`, `DryMsg`) and respect concurrency where parallel work occurs. Any action that changes remote hosts must be wrapped so that no changes are made when `--dry-run` is active.
- Add unit tests and a focused smoke test target if the behavior impacts cluster state; update the CI matrix when adding OS/distro specifics.

## OS/Distro Support
- See CI matrix in `.github/workflows/smoke.yml` for tested distributions. When adding support, extend the matrix and add corresponding smoke tests.

## Dependencies
- Dependabot is enabled; manual bumps are fine.
- Main dependencies are `github.com/k0sproject/rig`, `github.com/urfave/cli`, `github.com/stretchr/testify`, `github.com/creasty/defaults`, and `github.com/sirupsen/logrus`.
- Rig and k0s are maintained by the same authors.

## Agent Checklist (Compact)
- Default transport: use `ssh` (Go `crypto/ssh`). Use `openSSH` only when testing that mode explicitly. Use `localhost` or mocks in unit tests; avoid any real host changes.
- New phases: make them idempotent, implement `Title() string` and `Run(ctx)`; use `Prepare`, `ShouldRun`, hooks, and `DryRun()` when appropriate.
- Dry-run: wrap mutating calls with `Manager.Wet(...)` and emit `DryMsg` so `--dry-run` prints an accurate plan without changing state.
- Concurrency: respect `Manager.Concurrency` and `ConcurrentUploads` in parallel work.
- Naming/style: file names `snake_case.go`, human-readable phase titles, follow Go formatting, keep CLI flags/env vars consistent with patterns in `cmd/`.
- Security: do not embed secrets; prefer SSH agent or `openSSH` over inline keys in examples; use private/test addresses in docs/tests.
- Docs/CI: update `README.md` when behavior/config changes; extend smoke matrix and add focused smoke tests for OS/distro changes.
