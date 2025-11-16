# Repository Guidelines

## Project Structure & Module Organization
`cmd/gwcli/` contains the CLI entrypoint and flag wiring, backed by reusable packages in `pkg/gwcli/` (Gmail API clients, label helpers) and `pkg/gpg/` (crypto utilities with existing tests). Agent trainings and references live in `claude-skill-gwcli/`; keep generated docs there if they target automation personas. Build artifacts land in `dist/` and can be wiped via `just clean`. Place new feature modules under `pkg/` next to the feature they extend so CLI-facing code stays thin.

## Build, Test, and Development Commands
Use the `just` recipes for repeatable workflows: `just build` compiles to `dist/gwcli`, `just build-version 1.2.3` injects version metadata, and `just run -- messages list` executes the CLI without installing. Quality gates are `just fmt`, `just vet`, and `just test`; `just check` runs vet followed by the test suite. For manual steps, `go build ./cmd/gwcli`, `go test ./...`, and `go run ./cmd/gwcli configure` behave the same and are safe to call inside scripts.

## Coding Style & Naming Conventions
All Go files must remain `gofmt`-clean (tabs for indentation, max 100-ish char lines). Keep package names short (`gwcli`, `gpg`), export only what the CLI needs, and prefer `func newThing()` factories over constructors that do real work. Flags and subcommands should use lowercase-hyphen names to match existing verbs (`messages list`, `gmailctl apply`). Run `just fmt` or `go fmt ./...` before sending a change.

## Testing Guidelines
Table-driven tests live beside the code as `*_test.go`. Today only `pkg/gpg/gpg_test.go` exists; mirror that structure for new packages and favor unit tests over end-to-end flows. Add regression coverage for every bugfix and ensure `go test ./... -cover` stays green before proposing a PR. When integration tests touch Gmail, gate them behind build tags and document required env vars.

## Commit & Pull Request Guidelines
Follow the existing log style: `<type>: summary` (`refactor: rename OAuth token store`) or a short imperative with optional issue/PR marker (`Add service account authentication (#2)`). Group related changes, include `just check` output snippets in the PR description, link any tracking issue, and attach sample command output or JSON diffs for user-visible changes. PRs should describe configuration impacts (e.g., new files in `~/.config/gwcli/`) and note any manual steps reviewers must perform.

## Security & Configuration Notes
OAuth credentials live in `~/.config/gwcli/`; never commit those JSON files or redact sensitive IDs in examples. Prefer environment variables or `.jsonnet` config files under the user’s home directory when writing docs or scripts. If you touch crypto or token-handling code in `pkg/gpg/`, add tests that exercise invalid key material and update the README if new scopes or secrets are required.
