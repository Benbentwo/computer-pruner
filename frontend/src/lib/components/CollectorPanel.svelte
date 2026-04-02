<script lang="ts">
  import { collectorItems } from '../stores'
  import { formatBytes } from '../utils/format'
  import type { CollectorItem } from '../types'

  let isExpanded = false

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

  function handleDeleteAll() {
    if (confirm(`Delete ${$collectorItems.length} item(s)? This action cannot be undone.`)) {
      // TODO: Dispatch delete action to backend
      collectorItems.set([])
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
      <span class="toggle-icon" class:rotated={isExpanded}>▼</span>
      <span class="header-title">
        Collector
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
                {#if item.isDir}
                  📁
                {:else}
                  📄
                {/if}
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
        <button
          class="delete-all-btn danger"
          on:click={handleDeleteAll}
          disabled={isEmpty}
        >
          Delete All
        </button>
      </div>
    </div>
  {/if}
</div>

<style>
  .collector-panel {
    display: flex;
    flex-direction: column;
    max-height: 150px;
    transition: max-height var(--transition);
  }

  .collector-panel.expanded {
    max-height: 400px;
  }

  .collector-panel.empty {
    opacity: 0.6;
  }

  .collector-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.75em 1em;
    background: transparent;
    border: none;
    border-bottom: 1px solid var(--border);
    cursor: pointer;
    color: var(--text-primary);
    font-weight: 500;
    font-size: 0.9em;
    transition: background-color var(--transition);
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
    transition: transform var(--transition);
    font-size: 0.8em;
  }

  .toggle-icon.rotated {
    transform: rotate(180deg);
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
    padding: 0.15em 0.4em;
    border-radius: 3px;
    font-size: 0.8em;
    font-weight: 600;
  }

  .total-size {
    color: var(--text-secondary);
    font-size: 0.9em;
    font-weight: 500;
  }

  .collector-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .items-list {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 0.5em;
    padding: 0.75em;
  }

  .collector-item {
    display: flex;
    align-items: center;
    gap: 0.5em;
    padding: 0.5em;
    background-color: var(--bg-tertiary);
    border-radius: 4px;
    font-size: 0.85em;
  }

  .item-info {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 0.5em;
    min-width: 0;
  }

  .item-icon {
    font-size: 1em;
    flex-shrink: 0;
  }

  .item-details {
    flex: 1;
    min-width: 0;
  }

  .item-name {
    color: var(--text-primary);
    font-weight: 500;
    truncate;
  }

  .item-path {
    color: var(--text-tertiary);
    margin-top: 0.15em;
    truncate;
  }

  .item-size {
    flex-shrink: 0;
    color: var(--text-secondary);
    font-weight: 500;
    font-size: 0.9em;
  }

  .remove-btn {
    flex-shrink: 0;
    width: 24px;
    height: 24px;
    padding: 0;
    background-color: transparent;
    border: 1px solid var(--border);
    color: var(--text-secondary);
    cursor: pointer;
    border-radius: 3px;
    font-size: 0.8em;
    transition: all var(--transition);
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
    padding: 0.75em;
    border-top: 1px solid var(--border);
    flex-shrink: 0;
    gap: 0.75em;
  }

  .summary {
    color: var(--text-secondary);
  }

  .delete-all-btn {
    flex-shrink: 0;
  }
</style>
