# Disk Analyzer — Claude Code Instructions

## Project Overview
Cross-platform disk space analyzer (DaisyDisk competitor) built with Wails v2.
- **Backend:** Go — filesystem scanning, file operations, volume enumeration
- **Frontend:** Svelte + TypeScript + D3.js — sunburst visualization, UI

## Architecture
- `main.go` / `app.go` — Wails app entry and lifecycle
- `internal/scanner/` — filesystem walk, tree building, progress streaming
- `internal/volume/` — mounted volume enumeration
- `internal/fileops/` — delete, reveal, preview operations
- `internal/preferences/` — app settings persistence
- `internal/platform/` — OS-specific helpers (protected paths, file manager, theme)
- `frontend/src/lib/components/` — Svelte UI components
- `frontend/src/lib/stores/` — reactive state (Svelte writable stores)
- `frontend/src/lib/types/` — TypeScript interfaces (mirrors Go structs)

## Build & Dev
```bash
wails dev          # hot-reload dev mode
wails build        # production binary
```

## Conventions
- Go services expose methods to frontend via Wails bindings (auto-generated TS)
- Real-time data flows via `runtime.EventsEmit` (Go→JS) and `runtime.EventsOn` (JS listener)
- Event names: `scan:progress`, `scan:complete`, `scan:error`, `delete:progress`, `delete:complete`
- CSS theming via custom properties; dark mode is default
- Never hardcode colors in components — use `var(--*)` tokens from `app.css`

## Git Workflow
- `main` — stable, always builds
- `feature/*` — feature branches off main
- `fix/*` — bug fix branches off main
- Worktrees encouraged for parallel feature work
- Commit messages: imperative mood, concise ("Add volume enumeration", "Fix scan cancellation race")
