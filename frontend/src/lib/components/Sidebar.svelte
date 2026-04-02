<script lang="ts">
  import { hoveredNode } from '../stores'
  import { formatBytes } from '../utils/format'
  import type { TreeNode } from '../types'

  function formatDate(dateString?: string): string {
    if (!dateString) return '-'
    try {
      return new Date(dateString).toLocaleDateString()
    } catch {
      return '-'
    }
  }
</script>

<div class="sidebar">
  <!-- Hover Detail Panel -->
  <div class="detail-panel">
    {#if $hoveredNode}
      <div class="detail-content">
        <div class="detail-header">
          <div class="detail-icon">
            {#if $hoveredNode.isDir}
              <span class="icon">📁</span>
            {:else}
              <span class="icon">📄</span>
            {/if}
          </div>
          <h3 class="detail-name">{$hoveredNode.name}</h3>
        </div>

        <div class="detail-info">
          <div class="info-row">
            <span class="label">Size:</span>
            <span class="value font-semibold">{formatBytes($hoveredNode.size)}</span>
          </div>

          <div class="info-row">
            <span class="label">Path:</span>
            <span class="value truncate" title={$hoveredNode.path}>
              {$hoveredNode.path}
            </span>
          </div>

          {#if $hoveredNode.isDir && $hoveredNode.fileCount !== undefined}
            <div class="info-row">
              <span class="label">Files:</span>
              <span class="value">{$hoveredNode.fileCount}</span>
            </div>
          {/if}

          {#if $hoveredNode.isDir && $hoveredNode.dirCount !== undefined}
            <div class="info-row">
              <span class="label">Dirs:</span>
              <span class="value">{$hoveredNode.dirCount}</span>
            </div>
          {/if}

          {#if $hoveredNode.modTime}
            <div class="info-row">
              <span class="label">Modified:</span>
              <span class="value text-sm">{formatDate($hoveredNode.modTime)}</span>
            </div>
          {/if}

          {#if $hoveredNode.isProtected}
            <div class="protected-badge">
              <span class="text-xs">Protected</span>
            </div>
          {/if}
        </div>
      </div>
    {:else}
      <div class="detail-placeholder">
        <p class="text-tertiary text-sm">Hover over a sector to see details</p>
      </div>
    {/if}
  </div>

  <!-- File List Section (Placeholder) -->
  <div class="file-list-section">
    <div class="section-header">
      <h4>Files & Directories</h4>
    </div>
    <div class="file-list-placeholder">
      <p class="text-tertiary text-sm">Select an item to view contents</p>
    </div>
  </div>
</div>

<style>
  .sidebar {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: 1em;
    gap: 1em;
    overflow-y: auto;
  }

  .detail-panel {
    flex: 0 0 auto;
    background-color: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 1em;
    min-height: 120px;
  }

  .detail-placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    text-align: center;
  }

  .detail-content {
    display: flex;
    flex-direction: column;
    gap: 0.75em;
  }

  .detail-header {
    display: flex;
    align-items: flex-start;
    gap: 0.75em;
  }

  .detail-icon {
    flex: 0 0 auto;
    font-size: 1.5em;
  }

  .detail-name {
    margin: 0;
    font-size: 0.95em;
    color: var(--text-primary);
    word-break: break-word;
  }

  .detail-info {
    display: flex;
    flex-direction: column;
    gap: 0.5em;
  }

  .info-row {
    display: flex;
    justify-content: space-between;
    font-size: 0.85em;
  }

  .label {
    color: var(--text-secondary);
    font-weight: 500;
    flex: 0 0 auto;
    margin-right: 0.5em;
  }

  .value {
    color: var(--text-primary);
    flex: 1;
    text-align: right;
    word-break: break-word;
  }

  .protected-badge {
    display: inline-flex;
    background-color: var(--warning);
    color: var(--bg-primary);
    padding: 0.25em 0.5em;
    border-radius: 3px;
    width: fit-content;
    margin-top: 0.25em;
    font-weight: 600;
  }

  .file-list-section {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .section-header {
    padding-bottom: 0.75em;
    border-bottom: 1px solid var(--border);
    flex: 0 0 auto;
  }

  .section-header h4 {
    margin: 0;
    font-size: 0.9em;
    color: var(--text-secondary);
  }

  .file-list-placeholder {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    text-align: center;
  }
</style>
