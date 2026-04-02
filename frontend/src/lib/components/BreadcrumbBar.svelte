<script lang="ts">
  import { breadcrumbs, currentRoot } from '../stores'
  import type { TreeNode } from '../types'

  function handleCrumbClick(node: TreeNode | null) {
    if (node) {
      currentRoot.set(node)
    } else {
      // Click on "Root" - reset to top
      currentRoot.set(null)
    }
    // TODO: Also update breadcrumbs array appropriately
  }
</script>

<div class="breadcrumb-bar">
  <button class="crumb" on:click={() => handleCrumbClick(null)}>
    Root
  </button>

  {#each $breadcrumbs as crumb, idx (idx)}
    <span class="separator">/</span>
    <button class="crumb" on:click={() => handleCrumbClick(crumb)}>
      {crumb.name}
    </button>
  {/each}
</div>

<style>
  .breadcrumb-bar {
    display: flex;
    align-items: center;
    gap: 0.25em;
    overflow-x: auto;
    padding: 0.5em 0;
  }

  .crumb {
    padding: 0.35em 0.75em;
    background: transparent;
    border: none;
    color: var(--accent);
    cursor: pointer;
    font-size: 0.9em;
    white-space: nowrap;
    transition: color var(--transition);
  }

  .crumb:hover {
    color: var(--accent-hover);
    text-decoration: underline;
  }

  .separator {
    color: var(--text-tertiary);
    margin: 0 0.25em;
  }
</style>
