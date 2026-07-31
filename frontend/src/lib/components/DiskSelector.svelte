<script lang="ts">
  import { onMount } from 'svelte'
  import { volumes } from '../stores'
  import { formatBytes } from '../utils/format'
  import { createEventDispatcher } from 'svelte'
  import { listVolumes, listDirectory } from '../utils/wailsBindings'
  import type { DirEntry } from '../utils/wailsBindings'
  import type { VolumeInfo } from '../types'

  const dispatch = createEventDispatcher<{ scan: {path: string, forceRescan: boolean} }>()

  let loading = true
  let error = ''
  let browsingPath = '' // empty = show volumes
  let browsingEntries: DirEntry[] = []
  let browsingLoading = false
  let pathSegments: string[] = []

  onMount(async () => {
    try {
      const vols = await listVolumes()
      volumes.set(vols)
    } catch (e: any) {
      error = e?.message || 'Failed to load volumes'
    } finally {
      loading = false
    }
  })

  function getUsagePercent(volume: VolumeInfo): number {
    if (volume.totalBytes === 0) return 0
    return (volume.usedBytes / volume.totalBytes) * 100
  }

  function getBarColor(percent: number): string {
    if (percent < 70) return 'var(--bar-green)'
    if (percent < 90) return 'var(--bar-yellow)'
    return 'var(--bar-red)'
  }

  async function navigateTo(path: string) {
    browsingPath = path
    browsingLoading = true
    pathSegments = path === '/' ? ['/'] : path.split('/').filter(Boolean)
    if (path !== '/') pathSegments.unshift('/')

    try {
      browsingEntries = await listDirectory(path)
      // Sort: dirs first, then by name
      browsingEntries.sort((a, b) => {
        if (a.isDir !== b.isDir) return a.isDir ? -1 : 1
        return a.name.localeCompare(b.name)
      })
    } catch (e: any) {
      browsingEntries = []
      error = e?.message || 'Failed to read directory'
    } finally {
      browsingLoading = false
    }
  }

  function navigateToSegment(idx: number) {
    if (idx === 0) {
      goBackToVolumes()
      return
    }
    const path = '/' + pathSegments.slice(1, idx + 1).join('/')
    navigateTo(path)
  }

  function goBackToVolumes() {
    browsingPath = ''
    browsingEntries = []
    pathSegments = []
    error = ''
  }

  function handleScanCurrent(forceRescan: boolean = false) {
    if (browsingPath) {
      dispatch('scan', {path: browsingPath, forceRescan})
    }
  }

  function handleScanVolume(mountPoint: string, forceRescan: boolean = false) {
    dispatch('scan', {path: mountPoint, forceRescan})
  }
</script>

<div class="selector">
  <div class="selector-header">
    <div class="app-icon">💾</div>
    <h2>ComputerPruner</h2>
    <p class="subtitle">Select a volume or navigate to a folder to analyze</p>
  </div>

  {#if browsingPath}
    <!-- Folder Browser Mode -->
    <div class="browser">
      <div class="browser-toolbar">
        <div class="breadcrumb-trail">
          <button class="crumb volumes-crumb" on:click={goBackToVolumes} data-tooltip="Back to volume list">
            ← Volumes
          </button>
          {#each pathSegments as seg, idx}
            <span class="sep">›</span>
            <button class="crumb" on:click={() => navigateToSegment(idx)}>
              {seg === '/' ? '/' : seg}
            </button>
          {/each}
        </div>

        <div class="actions">
          <button class="scan-btn secondary" on:click={() => handleScanCurrent(true)} data-tooltip="Force Rescan ignoring cache">
            ⟳
          </button>
          <button class="scan-btn primary" on:click={() => handleScanCurrent(false)} data-tooltip="Scan this folder">
            ▶ Scan Here
          </button>
        </div>
      </div>

      {#if browsingLoading}
        <div class="browser-loading">
          <div class="loading-spinner" />
        </div>
      {:else if browsingEntries.length === 0}
        <div class="browser-empty">
          <p class="text-tertiary">Empty directory</p>
        </div>
      {:else}
        <div class="browser-entries">
          {#each browsingEntries as entry (entry.path)}
            <button
              class="entry-row"
              class:is-dir={entry.isDir}
              on:click={() => entry.isDir ? navigateTo(entry.path) : null}
              on:dblclick={() => !entry.isDir ? null : null}
            >
              <span class="entry-icon">
                {#if entry.isDir}📁{:else}📄{/if}
              </span>
              <span class="entry-name" title={entry.name}>{entry.name}</span>
              {#if !entry.isDir}
                <span class="entry-size">{formatBytes(entry.size)}</span>
              {/if}
              {#if entry.isDir}
                <span class="entry-arrow">›</span>
              {/if}
            </button>
          {/each}
        </div>
      {/if}
    </div>
  {:else}
    <!-- Volume List Mode -->
    {#if loading}
      <div class="loading-state">
        <div class="loading-spinner" />
        <p class="text-tertiary">Discovering volumes…</p>
      </div>
    {:else if error}
      <div class="error-state">
        <p class="text-secondary">{error}</p>
        <button on:click={() => location.reload()}>Retry</button>
      </div>
    {:else}
      <div class="volumes-list">
        {#each $volumes as volume (volume.mountPoint)}
          {@const pct = getUsagePercent(volume)}
          <div class="volume-row">
            <button class="volume-info" on:click={() => navigateTo(volume.mountPoint)}>
              <span class="vol-icon">
                {#if volume.mountPoint === '/'}💻{:else}💽{/if}
              </span>

              <div class="vol-details">
                <div class="vol-top-line">
                  <span class="vol-name">{volume.name}</span>
                  <span class="vol-fs text-tertiary">{volume.fsType.toUpperCase()}</span>
                </div>
                <div class="vol-path text-tertiary">{volume.mountPoint}</div>
                <div class="vol-bar-container">
                  <div class="vol-bar" style="width: {pct}%; background: {getBarColor(pct)}" />
                </div>
                <div class="vol-capacity">
                  <span class="text-sm">{formatBytes(volume.usedBytes)} / {formatBytes(volume.totalBytes)}</span>
                  <span class="text-secondary text-sm">{formatBytes(volume.freeBytes)} free</span>
                </div>
              </div>
            </button>

            <div class="vol-actions">
              <button
                class="vol-scan-btn rescan"
                on:click|stopPropagation={() => handleScanVolume(volume.mountPoint, true)}
                data-tooltip="Force Rescan volume ignoring cache"
              >
                ⟳
              </button>
              <button
                class="vol-scan-btn"
                on:click|stopPropagation={() => handleScanVolume(volume.mountPoint, false)}
                data-tooltip="Scan entire volume"
              >
                ▶
              </button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<style>
  .selector {
    width: 100%;
    max-width: 720px;
    display: flex;
    flex-direction: column;
    gap: 1.5em;
  }

  .selector-header {
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.35em;
  }

  .app-icon { font-size: 2.5em; }

  .selector-header h2 {
    margin: 0;
    font-size: 1.6em;
    color: var(--text-primary);
  }

  .subtitle {
    margin: 0;
    color: var(--text-secondary);
    font-size: 0.95em;
  }

  /* ── Loading / Error ──────────────────── */

  .loading-state, .error-state {
    padding: 3em;
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1em;
  }

  .loading-spinner {
    width: 28px; height: 28px;
    border: 3px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin { to { transform: rotate(360deg); } }

  /* ── Volume List ──────────────────────── */

  .volumes-list {
    display: flex;
    flex-direction: column;
    gap: 0.4em;
  }

  .volume-row {
    display: flex;
    align-items: stretch;
    gap: 0;
    background-color: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 10px;
    overflow: hidden;
    transition: border-color 200ms ease;
  }

  .volume-row:hover {
    border-color: var(--accent);
  }

  .volume-info {
    flex: 1;
    display: flex;
    gap: 0.75em;
    align-items: center;
    padding: 0.85em 1em;
    background: transparent;
    border: none;
    cursor: pointer;
    text-align: left;
    color: var(--text-primary);
    transition: background-color 150ms ease;
  }

  .volume-info:hover {
    background-color: var(--bg-tertiary);
  }

  .vol-icon { font-size: 1.6em; flex-shrink: 0; }

  .vol-details {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.25em;
  }

  .vol-top-line {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
  }

  .vol-name { font-weight: 600; font-size: 0.95em; }
  .vol-fs { font-size: 0.65em; font-weight: 600; letter-spacing: 0.05em; }
  .vol-path { font-size: 0.75em; }

  .vol-bar-container {
    height: 4px;
    background-color: var(--bg-tertiary);
    border-radius: 2px;
    overflow: hidden;
  }

  .vol-bar {
    height: 100%;
    border-radius: 2px;
    transition: width 400ms ease;
  }

  .vol-capacity {
    display: flex;
    justify-content: space-between;
    font-size: 0.78em;
  }

  .vol-actions {
    display: flex;
    flex-direction: row;
    border-left: 1px solid var(--border);
  }

  .vol-scan-btn {
    flex: 0 0 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: transparent;
    border: none;
    cursor: pointer;
    font-size: 1.1em;
    color: var(--text-secondary);
    transition: all 150ms ease;
  }

  .vol-scan-btn.rescan {
    border-right: 1px solid var(--border);
    font-size: 1.25em;
  }

  .vol-scan-btn:hover {
    background-color: var(--accent);
    color: white;
  }

  /* ── Browser ──────────────────────────── */

  .browser {
    display: flex;
    flex-direction: column;
    background-color: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 10px;
    max-height: 420px;
  }

  .browser-toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.5em 0.75em;
    border-bottom: 1px solid var(--border);
    gap: 0.5em;
    flex-shrink: 0;
    overflow: visible;
    border-radius: 10px 10px 0 0;
  }

  .breadcrumb-trail {
    display: flex;
    align-items: center;
    gap: 0.15em;
    overflow-x: auto;
    flex: 1;
    min-width: 0;
  }

  .crumb {
    padding: 0.2em 0.4em;
    background: transparent;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 0.8em;
    border-radius: 3px;
    white-space: nowrap;
    transition: all 100ms ease;
  }

  .crumb:hover { color: var(--accent); background-color: var(--bg-tertiary); }
  .crumb:last-child { color: var(--text-primary); font-weight: 600; }

  .volumes-crumb {
    font-weight: 500;
    color: var(--text-secondary);
    border: 1px solid var(--border);
    padding: 0.2em 0.5em;
    border-radius: 4px;
  }

  .volumes-crumb:hover {
    background-color: var(--accent);
    border-color: var(--accent);
    color: white;
  }

  .sep { color: var(--text-tertiary); font-size: 0.8em; }

  .actions {
    display: flex;
    gap: 0.5em;
    align-items: center;
  }

  .scan-btn {
    flex-shrink: 0;
    padding: 0.4em 0.9em;
    font-size: 0.8em;
    font-weight: 600;
    border-radius: 6px;
    white-space: nowrap;
  }

  .scan-btn.secondary {
    background: transparent;
    border: 1px solid var(--border);
    color: var(--text-secondary);
    font-size: 1em;
    padding: 0.3em 0.6em;
  }

  .scan-btn.secondary:hover {
    color: var(--text-primary);
    border-color: var(--text-tertiary);
  }

  .browser-loading, .browser-empty {
    padding: 2em;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .browser-entries {
    overflow-y: auto;
    overflow-x: hidden;
    display: flex;
    flex-direction: column;
  }

  .entry-row {
    display: flex;
    align-items: center;
    gap: 0.5em;
    padding: 0.45em 0.85em;
    background: transparent;
    border: none;
    border-bottom: 1px solid var(--border);
    cursor: default;
    text-align: left;
    color: var(--text-primary);
    font-size: 0.85em;
    transition: background-color 100ms ease;
  }

  .entry-row.is-dir { cursor: pointer; }
  .entry-row.is-dir:hover { background-color: var(--bg-tertiary); }
  .entry-row:last-child { border-bottom: none; }

  .entry-icon { flex-shrink: 0; font-size: 0.95em; }

  .entry-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .entry-size {
    flex-shrink: 0;
    color: var(--text-tertiary);
    font-size: 0.85em;
    font-variant-numeric: tabular-nums;
  }

  .entry-arrow {
    flex-shrink: 0;
    color: var(--text-tertiary);
    font-size: 0.9em;
  }

  /* ── Utility ──────────────────────────── */

  :root {
    --bar-green: linear-gradient(90deg, hsl(140, 60%, 40%), hsl(140, 60%, 50%));
    --bar-yellow: linear-gradient(90deg, hsl(45, 80%, 45%), hsl(45, 80%, 55%));
    --bar-red: linear-gradient(90deg, hsl(0, 70%, 45%), hsl(0, 70%, 55%));
  }
</style>
