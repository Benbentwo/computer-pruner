## What changed

<!-- One or two sentences. -->

## Why

<!-- The problem this solves. Link the issue if there is one: Fixes #123 -->

## How it was tested

<!-- Commands run, cases exercised, anything checked by hand. -->

## Verified on

- [ ] macOS
- [ ] Windows
- [ ] Not platform-specific (typecheck + `go test ./internal/...` only)

## Checklist

- [ ] `gofmt -l .` is clean, and `go vet ./...` passes for both `GOOS=darwin` and `GOOS=windows`
- [ ] `go test ./internal/...` passes
- [ ] **Any change to a destructive operation — delete, trash, or protected-path
      logic — comes with a test.** Nothing that can remove a user's files ships
      unguarded.
- [ ] No private file paths, usernames or personal directory names in the diff,
      the description, or any attached screenshot
