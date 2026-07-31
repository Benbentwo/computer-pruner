<script lang="ts">
  import { currentRoot, hoveredNode, collectorItems, breadcrumbs, scanTree } from '../stores'
  import { formatBytes } from '../utils/format'
  import { getNodeColor, getCategory } from '../utils/colorScale'
  import { revealInFileManager } from '../utils/wailsBindings'
  import type { TreeNode } from '../types'

  type SortMode = 'size' | 'name' | 'date'
  let sortBy: SortMode = 'size'

  $: children = getChildren($currentRoot, sortBy)

  function getChildren(root: TreeNode | null, sort: SortMode): TreeNode[] {
    if (!root || !root.children) return []

    const items = [...root.children]
    switch (sort) {
      case 'size':
        items.sort((a, b) => b.size - a.size)
        break
      case 'name':
        items.sort((a, b) => a.name.localeCompare(b.name))
        break
      case 'date':
        items.sort((a, b) => (b.modTime ?? '').localeCompare(a.modTime ?? ''))
        break
    }
    return items
  }

  function getPercent(child: TreeNode): number {
    const parent = $currentRoot
    if (!parent || parent.size === 0) return 0
    return (child.size / parent.size) * 100
  }

  function drillInto(node: TreeNode) {
    if (node.isDir && node.children && node.children.length > 0) {
      currentRoot.set(node)
      breadcrumbs.update(crumbs => [...crumbs, node])
    }
  }

  function addToCollector(node: TreeNode) {
    collectorItems.update(items => {
      if (items.find(i => i.path === node.path)) return items
      return [...items, {
        path: node.path,
        name: node.name,
        size: node.size,
        isDir: node.isDir,
      }]
    })
  }

  function handleReveal(path: string) {
    revealInFileManager(path)
  }

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
          <div class="detail-text">
            <h3 class="detail-name">{$hoveredNode.name}</h3>
            <span class="detail-category text-tertiary text-xs">{getCategory($hoveredNode)}</span>
          </div>
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
              <span class="value">{$hoveredNode.fileCount.toLocaleString()}</span>
            </div>
          {/if}

          {#if $hoveredNode.isDir && $hoveredNode.dirCount !== undefined}
            <div class="info-row">
              <span class="label">Dirs:</span>
              <span class="value">{$hoveredNode.dirCount.toLocaleString()}</span>
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
              <span class="text-xs">🔒 Protected</span>
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

  <!-- File List Section -->
  <div class="file-list-section">
    <div class="section-header">
      <h4>Contents</h4>
      <div class="sort-controls">
        <button class="sort-btn" class:active={sortBy === 'size'} on:click={() => sortBy = 'size'}>Size</button>
        <button class="sort-btn" class:active={sortBy === 'name'} on:click={() => sortBy = 'name'}>Name</button>
        <button class="sort-btn" class:active={sortBy === 'date'} on:click={() => sortBy = 'date'}>Date</button>
      </div>
    </div>

    <div class="file-list">
      {#if children.length === 0}
        <div class="file-list-placeholder">
          <p class="text-tertiary text-sm">No items to display</p>
        </div>
      {:else}
        {#each children as child (child.path)}
          <button
            class="file-row"
            on:click={() => drillInto(child)}
            on:mouseenter={() => hoveredNode.set(child)}
            on:mouseleave={() => hoveredNode.set(null)}
            on:contextmenu|preventDefault={() => addToCollector(child)}
          >
            <div class="file-color-bar" style="background: {getNodeColor(child, 1)}" />
            <span class="file-icon">
              {#if child.isDir}📁{:else}📄{/if}
            </span>
            <div class="file-details">
              <span class="file-name" title={child.name}>{child.name}</span>
              <div class="file-bar-container">
                <div class="file-bar" style="width: {getPercent(child)}%; background: {getNodeColor(child, 1)}" />
              </div>
            </div>
            <span class="file-size">{formatBytes(child.size)}</span>
          </button>
        {/each}
      {/if}
    </div>
  </div>
</div>

<style>
  .sidebar {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: 0.75em;
    gap: 0.75em;
    overflow: hidden;
  }

  .detail-panel {
    flex: 0 0 170px;
    background-color: var(--bg-tertiary);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.75em;
    overflow-y: auto;
  }

  .detail-placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    text-align: center;
    min-height: 60px;
  }

  .detail-content {
    display: flex;
    flex-direction: column;
    gap: 0.6em;
  }

  .detail-header {
    display: flex;
    align-items: flex-start;
    gap: 0.6em;
  }

  .detail-icon {
    flex: 0 0 auto;
    font-size: 1.3em;
  }

  .detail-text {
    flex: 1;
    min-width: 0;
  }

  .detail-name {
    margin: 0;
    font-size: 0.9em;
    color: var(--text-primary);
    word-break: break-word;
  }

  .detail-category {
    display: block;
    margin-top: 0.15em;
  }

  .detail-info {
    display: flex;
    flex-direction: column;
    gap: 0.35em;
  }

  .info-row {
    display: flex;
    justify-content: space-between;
    font-size: 0.8em;
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
    background-color: hsl(45, 80%, 50%);
    color: var(--bg-primary);
    padding: 0.2em 0.5em;
    border-radius: 4px;
    width: fit-content;
    margin-top: 0.15em;
    font-weight: 600;
  }

  /* File list */
  .file-list-section {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
  }

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding-bottom: 0.5em;
    border-bottom: 1px solid var(--border);
    flex: 0 0 auto;
  }

  .section-header h4 {
    margin: 0;
    font-size: 0.85em;
    color: var(--text-secondary);
  }

  .sort-controls {
    display: flex;
    gap: 2px;
  }

  .sort-btn {
    padding: 0.2em 0.5em;
    font-size: 0.7em;
    background: transparent;
    border: 1px solid var(--border);
    color: var(--text-tertiary);
    border-radius: 3px;
    cursor: pointer;
    transition: all 150ms ease;
  }

  .sort-btn.active {
    background-color: var(--accent);
    border-color: var(--accent);
    color: white;
  }

  .sort-btn:hover:not(.active) {
    border-color: var(--text-tertiary);
    color: var(--text-secondary);
  }

  .file-list {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    padding-top: 0.25em;
  }

  .file-list-placeholder {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    text-align: center;
    padding: 2em;
  }

  .file-row {
    display: flex;
    align-items: center;
    gap: 0.4em;
    padding: 0.35em 0.5em;
    background: transparent;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    text-align: left;
    color: var(--text-primary);
    font-size: 0.82em;
    transition: background-color 100ms ease;
    flex-shrink: 0;
  }

  .file-row:hover {
    background-color: var(--bg-tertiary);
  }

  .file-color-bar {
    width: 3px;
    height: 20px;
    border-radius: 1.5px;
    flex-shrink: 0;
  }

  .file-icon {
    font-size: 0.9em;
    flex-shrink: 0;
  }

  .file-details {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .file-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 500;
  }

  .file-bar-container {
    height: 2px;
    background-color: var(--bg-tertiary);
    border-radius: 1px;
    overflow: hidden;
  }

  .file-bar {
    height: 100%;
    border-radius: 1px;
    transition: width 300ms ease;
  }

  .file-size {
    flex-shrink: 0;
    color: var(--text-secondary);
    font-size: 0.85em;
    font-weight: 500;
    font-variant-numeric: tabular-nums;
  }
</style>
