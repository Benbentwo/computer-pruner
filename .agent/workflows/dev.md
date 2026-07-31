---
description: Start the Wails dev server with browser preview
---

## Start Dev Server

// turbo-all

1. From the repository root, start the Wails development server in browser mode:

```bash
wails dev -browser
```

The dev server will start and open the app in your default browser. The frontend URL is printed in the terminal output (typically `http://localhost:<port>`). The port changes each run — check the terminal output for the actual URL.

## Notes

- Hot-reload is enabled: frontend changes (Svelte/CSS/TS) update instantly
- Go backend changes require a recompile (Wails handles this automatically)
- The `-browser` flag opens the app in a browser window instead of the native Wails webview
- Press `Ctrl+C` in the terminal to stop the dev server
