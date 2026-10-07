# Repository Instructions

## Go Version

- Use Go `1.27.1`.
- Keep the module, toolchain configuration, CI, and local development commands consistent with Go `1.27.1`.
- Prefer the standard library unless a dependency is justified.
- Run formatting and validation before completing changes:
  - `gofmt`
  - `go vet ./...`
  - `go test ./...`

## Go Practices

- Follow idiomatic Go and standard Go project conventions.
- Keep functions focused and error handling explicit.
- Wrap errors with useful context using `%w` where appropriate.
- Avoid unnecessary abstractions, global mutable state, and reflection.
- Use contexts correctly for cancellation and timeouts.
- Do not ignore returned errors.
- Add tests for new or changed behavior.
- Keep exported APIs documented.
- Run `go mod tidy` only when dependency changes require it.

## Dependency Restrictions

- Do not install, add, update, remove, or download dependencies without the user's explicit suggestion or approval.
- Before changing dependencies, explain the reason and identify the proposed dependency and version; wait for user approval.
- Prefer the standard library and existing dependencies whenever possible.

## Filesystem Boundary

- Filesystem operations are strictly limited to this repository.
- Do not read, write, create, delete, rename, or inspect files outside this repository.
- Do not change directories outside this repository.
- Do not access parent directories, home-directory configuration, temporary files, or external paths.

## GitHub Restrictions

- Do not run GitHub-related commands.
- Do not use `gh`.
- Do not access GitHub APIs, GitHub URLs, or GitHub repositories.
- Do not clone, fetch, pull, push, or otherwise communicate with GitHub.
