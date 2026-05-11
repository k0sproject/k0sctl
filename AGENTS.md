# Repository Guidelines

## Project Shape
- `main.go` is the CLI entrypoint.
- `cmd/` defines CLI commands and flags using `urfave/cli/v2`.
- `action/` contains command-level workflows that compose phases.
- `phase/` contains orchestration phases and the phase manager.
- `configurer/` contains OS/distro-specific command and path behavior.
- `pkg/apis/k0sctl.k0sproject.io/v1beta1/` contains the config API types and validation.
- `pkg/`, `internal/`, `smoke-test/`, `examples/`, and `integration/` contain reusable packages, private helpers, smoke tests, examples, and integration helpers.

## What k0sctl Does
- Bootstraps and manages k0s clusters from `k0sctl.yaml`.
- Connects to hosts primarily over SSH through `github.com/k0sproject/rig`; Windows workers may use SSH or WinRM. Rig is maintained by the same team and provides a consistent interface for remote execution, file transfer, and parallelism.
- Main flows include `apply`, `reset`, `init`, `kubeconfig`, `backup`, and dynamic config edit/status commands.
- Config flow: parse YAML into `v1beta1.Cluster`, apply defaults, validate, then run phases through `phase.Manager`.

## Development Commands
- Build: `make k0sctl`.
- Cross-build: `make build-all`.
- Unit tests: `make test` or `go test -v ./...`.
- Lint: `make lint`.
- Smoke tests: inspect `Makefile`, `smoke-test/Makefile`, and `.github/workflows/smoke.yml` for current targets and matrix.
- Use the Go/toolchain version declared in `go.mod`

## Testing Rules
- Unit tests must be hermetic: do not connect to real hosts, mutate remote state, rely on external network resources, or depend on host-specific config. Prefer mocks/fakes for unit tests.
- For end-to-end and smoke coverage, prefer the native `ssh` transport (`ssh:` / Go `crypto/ssh`).
- Use `openSSH:` only when explicitly testing the OpenSSH-client transport.
- Name tests after behavior, e.g. `TestValidateHosts_*`.

## Phase Manager Rules
- New phases must be idempotent, have human-readable titles, and live in `snake_case.go` files matching the phase intent. The struct name must be `PascalCase` matching the intent.
- Wire phases from the relevant action by adding them to the manager in the correct order.
- Respect `Manager.Concurrency` and `ConcurrentUploads` when doing parallel host work or uploads.

## Phase Implementation
- Embed `GenericPhase` in every new phase struct — it provides `Prepare`, `Wet`, `DryMsg`, `DryMsgf`, `parallelDo`, `parallelDoUpload`, and `SetManager`.
- Use `p.parallelDo(ctx, hosts, func(ctx, h) error { ... })` for concurrent per-host work; it respects `Manager.Concurrency` automatically.
- Use `p.parallelDoUpload(ctx, hosts, ...)` for file-transfer steps; it additionally respects `Manager.ConcurrentUploads`.
- `Wet(host, dryMsg, mutatingFuncs...)` runs the functions only in wet mode and prints `dryMsg` in dry-run mode — use it for every remote mutation.

## Dry-Run Rules
- Any action that changes remote hosts must be guarded so `--dry-run` makes no remote changes.
- Use `Wet(host, dryMsg, funcs...)` and `DryMsg(host, msg)` (both on `GenericPhase`) so dry-run output is an accurate per-host plan.
- If a phase needs alternate dry-run behavior, implement the dry-run interface instead of partially running mutating logic.

## Logging
- Use `log "github.com/sirupsen/logrus"` — it is the only logger in this project.
- Per-host messages must prefix the host: `log.Infof("%s: doing thing", h)`.
- Use `log.Debug`/`log.Debugf` for internal state, `log.Info`/`log.Infof` for user-visible progress, `log.Warn`/`log.Warnf` for recoverable problems.
- Do not use `fmt.Print*` for diagnostic output.

## Error Wrapping
- Wrap errors with context using `fmt.Errorf("doing X: %w", err)`.
- Do not re-wrap the same message twice up the call stack.

## Config API Changes
When adding or changing fields in `pkg/apis/k0sctl.k0sproject.io/v1beta1/`:
1. Update defaults (`creasty/defaults` struct tags or `SetDefaults` methods).
2. Update `Validate()` if the field has invariants.
3. Update `README.md` with the field name, type, default, and description.
4. Add a unit test in the same package.

## Transport And Platform Notes
- YAML keys: `ssh:` selects the native Go SSH transport, `openSSH:` uses the local OpenSSH client, `localhost:` targets the local machine, and `winRM:` is for Windows workers using WinRM, but SSH also functions on Windows if configured.
- Linux is the primary target. Windows support is limited to worker nodes and requires compatible k0s versions; check `README.md` and config validation before changing Windows behavior.
- Rig v0.x is the dependency line used here. Rig v2 API will be migrated to in the future.

## Docs, CI, And Security
- `README.md` is the user-facing source of truth for config fields and behavior. Update it when behavior, flags, or config schema changes.
- Update examples only when they are relevant to the behavior being changed.
- DCO is required: commits must be signed off with `git commit -s`.
- Keep secrets, real cluster addresses, and private keys out of docs, examples, and tests. Use private/test addresses and redacted or generated key material.
- Before assuming CI requirements, inspect `.github/workflows/`; workflows include more than unit tests and smoke tests.

## Agent Token Discipline
- Prefer narrow reads first: use `rg`, `git diff --stat`, `git diff --name-only`, targeted `sed`, and package-level tests before reading whole files, full diffs, or full logs.
- Keep tool output capped. Start with small `max_output_tokens` values, then rerun narrower commands if more context is needed.
- Avoid dumping raw smoke logs, full PR threads, or full `go test ./...` output into context. Search for specific failures, phases, files, or review comments instead.
- Preserve useful discoveries in agent memory, `AGENTS.md`, or a focused note before ending a large investigation, so future sessions do not repeat the same repo sweep.
- Start a fresh session after a substantial phase of work, especially after long PR review/comment loops, and seed it with only the current branch, failing check, unresolved comment, and relevant files.
