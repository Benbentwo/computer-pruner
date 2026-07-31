# ComputerPruner — Claude Code Instructions

## Project Overview
ComputerPruner is a cross-platform computer maintenance toolkit built with Wails v2.
- **Module:** `github.com/Benbentwo/computer-pruner`
- **Binary:** `computer-pruner`
- **Backend:** Go — filesystem scanning, file operations, volume enumeration
- **Frontend:** Svelte 4 + TypeScript + D3.js — sunburst visualization, UI
- **Targets:** macOS (amd64, arm64) and Windows (amd64, arm64). Linux is not a release target,
  but the code should still typecheck on linux where that is cheap.

## Scope
Only the **disk analyzer** pillar is implemented. Three further pillars are planned but
deliberately NOT built yet — do not implement them unless a task says so:
startup-app management, hardware/spec inventory, driver validation.

## Architecture
- `main.go` / `app.go` — Wails app entry, lifecycle, directory dialogs
- `internal/scanner/` — filesystem walk, tree building, progress streaming, scan cache
- `internal/volume/` — mounted volume enumeration
- `internal/fileops/` — delete, reveal, preview operations
- `internal/preferences/` — app settings persistence
- `internal/platform/` — OS-specific helpers (protected paths, file manager, theme)
- `frontend/src/lib/components/` — Svelte UI components
- `frontend/src/lib/stores/` — reactive state (Svelte writable stores)
- `frontend/src/lib/types/` — TypeScript interfaces (mirrors Go structs)
- `frontend/wailsjs/` — generated Wails bindings (gitignored, never hand-edit)
- `build/darwin/` — macOS `Info.plist` / `Info.dev.plist` bundle templates

Internal imports use the full module path, e.g.
`github.com/Benbentwo/computer-pruner/internal/platform`.

## Build & Dev
```bash
wails dev          # hot-reload dev mode
wails build        # production binary -> build/bin/computer-pruner
go vet ./...       # typecheck; also run with GOOS=darwin and GOOS=windows
go test ./internal/...
```
Release binaries are produced by GoReleaser and published to GitHub Releases.

## Conventions
- Go services expose methods to frontend via Wails bindings (auto-generated TS)
- Real-time data flows via `runtime.EventsEmit` (Go→JS) and `runtime.EventsOn` (JS listener)
- Event names: `scan:progress`, `scan:complete`, `scan:error`, `delete:progress`, `delete:complete`
- Platform-specific code goes in `internal/platform` behind build tags, not inline `runtime.GOOS` checks
- CSS theming via custom properties; dark mode is default
- Never hardcode colors in components — use `var(--*)` tokens from `app.css`

## Git Workflow
- `main` — stable, always builds
- `feature/*` — feature branches off main
- `fix/*` — bug fix branches off main
- Worktrees encouraged for parallel feature work
- Commit messages: imperative mood, concise ("Add volume enumeration", "Fix scan cancellation race")
