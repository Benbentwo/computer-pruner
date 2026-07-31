<script lang="ts">
  import { breadcrumbs, currentRoot, scanTree } from '../stores'
  import type { TreeNode } from '../types'

  function handleCrumbClick(node: TreeNode) {
    currentRoot.set(node)
    // Truncate breadcrumbs to this node
    breadcrumbs.update(crumbs => {
      const idx = crumbs.findIndex(c => c.path === node.path)
      if (idx >= 0) {
        return crumbs.slice(0, idx + 1)
      }
      return crumbs
    })
  }

  function goUp() {
    breadcrumbs.update(crumbs => {
      if (crumbs.length <= 1) return crumbs
      const newCrumbs = crumbs.slice(0, -1)
      currentRoot.set(newCrumbs[newCrumbs.length - 1])
      return newCrumbs
    })
  }
</script>

<div class="breadcrumb-bar">
  <button class="back-btn" on:click={goUp} title="Go up one level" disabled={$breadcrumbs.length <= 1}>
    ←
  </button>

  {#each $breadcrumbs as crumb, idx (crumb.path + idx)}
    {#if idx > 0}
      <span class="separator">›</span>
    {/if}
    <button
      class="crumb"
      class:active={idx === $breadcrumbs.length - 1}
      on:click={() => handleCrumbClick(crumb)}
    >
      {crumb.name}
    </button>
  {/each}
</div>

<style>
  .breadcrumb-bar {
    display: flex;
    align-items: center;
    gap: 0.15em;
    overflow-x: auto;
    padding: 0.25em 0;
  }

  .back-btn {
    padding: 0.3em 0.6em;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 0.9em;
    border-radius: 4px;
    transition: all 150ms ease;
    flex-shrink: 0;
    margin-right: 0.25em;
  }

  .back-btn:hover:not(:disabled) {
    background-color: var(--accent);
    border-color: var(--accent);
    color: white;
  }

  .back-btn:disabled {
    opacity: 0.3;
    cursor: default;
  }

  .crumb {
    padding: 0.25em 0.5em;
    background: transparent;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 0.85em;
    white-space: nowrap;
    transition: color 150ms ease;
    border-radius: 3px;
  }

  .crumb:hover {
    color: var(--accent);
    background-color: var(--bg-tertiary);
  }

  .crumb.active {
    color: var(--text-primary);
    font-weight: 600;
  }

  .separator {
    color: var(--text-tertiary);
    font-size: 0.85em;
    flex-shrink: 0;
  }
</style>
