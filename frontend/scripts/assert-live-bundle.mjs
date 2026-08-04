// Guards the production bundle against silently losing the Go bridge.
//
// With Svelte 4, resolving the 'svelte' import to the SSR runtime turns
// onMount into a no-op, and Rollup then dead-code-eliminates every lifecycle
// callback — including the ListVolumes call and the scan event listeners. The
// app still renders, but it sits on "Discovering volumes…" forever. That is a
// build-configuration failure (it happened with Vite 6 + vite-plugin-svelte 3),
// not a runtime one, so it must fail the build, not QA.
import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'

const assetsDir = new URL('../dist/assets/', import.meta.url).pathname
const js = readdirSync(assetsDir)
  .filter((f) => f.endsWith('.js'))
  .map((f) => readFileSync(join(assetsDir, f), 'utf8'))
  .join('\n')

// Each marker only survives bundling if the code that uses it does.
const markers = [
  'ListVolumes', // volume listing binding — DiskSelector's onMount
  'scan:complete', // event bridge — App's onMount via initEventBridge
]

const missing = markers.filter((m) => !js.includes(m))
if (missing.length > 0) {
  console.error(
    `assert-live-bundle: production bundle is missing ${missing.map((m) => `"${m}"`).join(', ')}.\n` +
      'The svelte lifecycle callbacks were dead-code-eliminated — the app would render but never call the Go backend.\n' +
      "Usual cause: 'svelte' resolved to its SSR runtime (no-op onMount) because the Vite major does not match @sveltejs/vite-plugin-svelte."
  )
  process.exit(1)
}
console.log('assert-live-bundle: Go bridge calls present in bundle')
