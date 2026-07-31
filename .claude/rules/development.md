---
description: Rules for building, running, and developing the Wails app
globs:
  - "**/*.go"
  - "**/*.svelte"
  - "**/*.ts"
  - "wails.json"
  - "go.mod"
  - "frontend/package.json"
---

# Development Tasks

## Running the Dev Server
- Use `wails dev` to start the app in hot-reload mode
- This launches both the Go backend and the Svelte frontend dev server
- The frontend dev server URL is auto-detected (`"frontend:dev:serverUrl": "auto"` in wails.json)
- Frontend changes hot-reload automatically; Go changes trigger a rebuild

## Building for Production
- Use `wails build` for a production binary
- Output: `build/bin/computer-pruner`
- Frontend is built via `npm run build` (configured in wails.json)

## Frontend Development
- Frontend lives in `frontend/`
- Package manager: npm
- Install deps: `cd frontend && npm install`
- Wails auto-runs `npm install` on `wails dev`/`wails build` via `"frontend:install"` config

## Go Backend Development
- Module: `github.com/Benbentwo/computer-pruner` (Go 1.25)
- All internal packages under `internal/`
- Run Go tests: `go test ./...`
- Wails bindings are auto-generated from exported methods on bound structs

## Wails Bindings
- When you add/change a public method on `App` (or any bound struct), Wails regenerates TypeScript bindings automatically during `wails dev`/`wails build`
- Generated bindings land in `frontend/wailsjs/`
- Do NOT manually edit files in `frontend/wailsjs/` — they are overwritten on rebuild

## Common Issues
- If the frontend fails to connect, ensure no other process is using the Wails dev port
- If Go changes don't reflect, restart `wails dev` (the Go watcher can miss some changes in `internal/`)
- If TypeScript shows missing binding types, run `wails generate module` to force-regenerate
- Full Disk Access permission may be needed on macOS for scanning system volumes
