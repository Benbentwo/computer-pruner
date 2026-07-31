<script lang="ts">
  import { collectorItems } from '../stores'
  import { formatBytes } from '../utils/format'
  import { deletePaths } from '../utils/wailsBindings'
  import type { CollectorItem } from '../types'

  let isExpanded = false
  let isDeleting = false

  $: totalSize = $collectorItems.reduce((sum, item) => sum + item.size, 0)
  $: isEmpty = $collectorItems.length === 0

  function toggleExpanded() {
    if (!isEmpty) {
      isExpanded = !isExpanded
    }
  }

  function removeItem(path: string) {
    collectorItems.update((items) => items.filter((item) => item.path !== path))
  }

  function clearAll() {
    collectorItems.set([])
    isExpanded = false
  }

  async function handleDeleteAll() {
    if (!confirm(`Move ${$collectorItems.length} item(s) to Trash?\nThis will free approximately ${formatBytes(totalSize)}.`)) {
      return
    }

    isDeleting = true
    try {
      const paths = $collectorItems.map(item => item.path)
      await deletePaths(paths)
      collectorItems.set([])
      isExpanded = false
    } catch (e: any) {
      alert('Delete failed: ' + (e?.message || 'Unknown error'))
    } finally {
      isDeleting = false
    }
  }
</script>

<div class="collector-panel" class:expanded={isExpanded} class:empty={isEmpty}>
  <button
    class="collector-header"
    on:click={toggleExpanded}
    disabled={isEmpty}
  >
    <div class="header-left">
      <span class="toggle-icon" class:rotated={isExpanded}>▸</span>
      <span class="header-title">
        🗑️ Collector
        {#if !isEmpty}
          <span class="badge">{$collectorItems.length}</span>
        {/if}
      </span>
    </div>

    {#if !isEmpty}
      <span class="total-size">{formatBytes(totalSize)}</span>
    {/if}
  </button>

  {#if isExpanded && !isEmpty}
    <div class="collector-content">
      <div class="items-list">
        {#each $collectorItems as item (item.path)}
          <div class="collector-item">
            <div class="item-info">
              <span class="item-icon">
                {#if item.isDir}📁{:else}📄{/if}
              </span>
              <div class="item-details">
                <div class="item-name" title={item.name}>{item.name}</div>
                <div class="item-path text-tertiary text-xs" title={item.path}>
                  {item.path}
                </div>
              </div>
            </div>
            <div class="item-size">{formatBytes(item.size)}</div>
            <button
              class="remove-btn"
              on:click={() => removeItem(item.path)}
              title="Remove from collector"
            >
              ✕
            </button>
          </div>
        {/each}
      </div>

      <div class="collector-footer">
        <div class="summary">
          <span class="text-sm">
            {$collectorItems.length} item(s) · {formatBytes(totalSize)}
          </span>
        </div>
        <div class="footer-actions">
          <button class="clear-btn secondary" on:click={clearAll}>
            Clear
          </button>
          <button
            class="delete-all-btn danger"
            on:click={handleDeleteAll}
            disabled={isEmpty || isDeleting}
          >
            {#if isDeleting}Deleting…{:else}Move to Trash{/if}
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .collector-panel {
    display: flex;
    flex-direction: column;
    max-height: 48px;
    transition: max-height 300ms ease;
  }

  .collector-panel.expanded {
    max-height: 350px;
  }

  .collector-panel.empty {
    opacity: 0.5;
  }

  .collector-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.65em 1em;
    background: transparent;
    border: none;
    cursor: pointer;
    color: var(--text-primary);
    font-weight: 500;
    font-size: 0.85em;
    transition: background-color 150ms ease;
  }

  .collector-header:hover:not(:disabled) {
    background-color: var(--bg-tertiary);
  }

  .collector-header:disabled {
    cursor: default;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 0.5em;
  }

  .toggle-icon {
    display: inline-block;
    transition: transform 200ms ease;
    font-size: 0.8em;
  }

  .toggle-icon.rotated {
    transform: rotate(90deg);
  }

  .header-title {
    display: flex;
    align-items: center;
    gap: 0.5em;
  }

  .badge {
    display: inline-block;
    background-color: var(--accent);
    color: white;
    padding: 0.1em 0.4em;
    border-radius: 10px;
    font-size: 0.8em;
    font-weight: 700;
    min-width: 1.4em;
    text-align: center;
  }

  .total-size {
    color: var(--text-secondary);
    font-size: 0.85em;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
  }

  .collector-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border-top: 1px solid var(--border);
  }

  .items-list {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 0.35em;
    padding: 0.5em;
  }

  .collector-item {
    display: flex;
    align-items: center;
    gap: 0.5em;
    padding: 0.4em 0.5em;
    background-color: var(--bg-tertiary);
    border-radius: 6px;
    font-size: 0.82em;
  }

  .item-info {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 0.4em;
    min-width: 0;
  }

  .item-icon {
    font-size: 0.9em;
    flex-shrink: 0;
  }

  .item-details {
    flex: 1;
    min-width: 0;
  }

  .item-name {
    color: var(--text-primary);
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .item-path {
    color: var(--text-tertiary);
    margin-top: 0.1em;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .item-size {
    flex-shrink: 0;
    color: var(--text-secondary);
    font-weight: 500;
    font-size: 0.85em;
    font-variant-numeric: tabular-nums;
  }

  .remove-btn {
    flex-shrink: 0;
    width: 22px;
    height: 22px;
    padding: 0;
    background-color: transparent;
    border: 1px solid var(--border);
    color: var(--text-tertiary);
    cursor: pointer;
    border-radius: 4px;
    font-size: 0.7em;
    transition: all 150ms ease;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .remove-btn:hover {
    background-color: var(--danger);
    border-color: var(--danger);
    color: white;
  }

  .collector-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.5em 0.75em;
    border-top: 1px solid var(--border);
    flex-shrink: 0;
    gap: 0.5em;
  }

  .summary {
    color: var(--text-secondary);
  }

  .footer-actions {
    display: flex;
    gap: 0.4em;
  }

  .clear-btn {
    font-size: 0.8em;
    padding: 0.35em 0.75em;
  }

  .delete-all-btn {
    font-size: 0.8em;
    padding: 0.35em 0.75em;
    flex-shrink: 0;
  }
</style>
