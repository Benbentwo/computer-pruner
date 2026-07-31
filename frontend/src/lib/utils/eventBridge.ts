/**
 * Event bridge — initializes Wails event listeners and updates Svelte stores.
 * Call initEventBridge() once on app mount.
 */

import { scanProgress, scanTree, currentRoot, breadcrumbs, isScanning, notifications } from '../stores'
import { onEvent } from './wailsBindings'
import type { TreeNode, ScanProgress, DeleteResult } from '../types'

let initialized = false

export function initEventBridge() {
  if (initialized) return
  initialized = true

  // ─── Scan Events ────────────────────────────────────────────

  onEvent('scan:progress', (progress: ScanProgress) => {
    scanProgress.set(progress)
  })

  onEvent('scan:complete', (tree: TreeNode) => {
    scanTree.set(tree)
    currentRoot.set(tree)
    breadcrumbs.set([tree])
    isScanning.set(false)
    scanProgress.set(null)

    notifications.update(n => [
      ...n,
      {
        id: Date.now().toString(),
        type: 'success' as const,
        message: `Scan complete — ${tree.fileCount?.toLocaleString() ?? 0} files found`,
      }
    ])
  })

  onEvent('scan:error', (errorMsg: string) => {
    isScanning.set(false)
    scanProgress.set(null)

    notifications.update(n => [
      ...n,
      {
        id: Date.now().toString(),
        type: 'error' as const,
        message: `Scan failed: ${errorMsg}`,
      }
    ])
  })

  // ─── Delete Events ──────────────────────────────────────────

  onEvent('delete:progress', (progress: { current: number; total: number; currentPath: string }) => {
    // Could update a deleteProgress store here if needed
  })

  onEvent('delete:complete', (result: DeleteResult) => {
    const msg = result.errors.length > 0
      ? `Deleted ${result.deletedCount} items with ${result.errors.length} error(s)`
      : `Deleted ${result.deletedCount} items`

    notifications.update(n => [
      ...n,
      {
        id: Date.now().toString(),
        type: result.errors.length > 0 ? 'error' as const : 'success' as const,
        message: msg,
      }
    ])
  })
}
