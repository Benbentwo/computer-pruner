<script lang="ts">
  import { onMount } from 'svelte'
  import * as d3 from 'd3'
  import type { TreeNode } from '../types'
  import { currentRoot, hoveredNode } from '../stores'

  let svgElement: SVGSVGElement
  let containerWidth = 800
  let containerHeight = 600

  onMount(() => {
    // Set up resize observer
    const resizeObserver = new ResizeObserver((entries) => {
      for (let entry of entries) {
        const { width, height } = entry.contentRect
        containerWidth = width
        containerHeight = height
      }
    })

    const parentElement = svgElement.parentElement
    if (parentElement) {
      resizeObserver.observe(parentElement)
    }

    return () => {
      resizeObserver.disconnect()
    }
  })

  function renderSunburst(root: TreeNode | null) {
    if (!root || !svgElement) return

    // TODO: Implement sunburst rendering
    // 1. Set up D3 partition layout
    // 2. Create arc generator
    // 3. Bind data to paths
    // 4. Add click handlers for drill-down
    // 5. Add hover handlers for detail panel
    // 6. Add transitions for interactivity
  }

  $: if (svgElement && $currentRoot) {
    renderSunburst($currentRoot)
  }
</script>

<div class="chart-container">
  <svg bind:this={svgElement} width={containerWidth} height={containerHeight} />

  {#if !$currentRoot}
    <div class="placeholder">
      <p>Scan a volume to visualize disk usage</p>
    </div>
  {/if}
</div>

<style>
  .chart-container {
    position: relative;
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  svg {
    width: 100%;
    height: 100%;
  }

  .placeholder {
    position: absolute;
    display: flex;
    align-items: center;
    justify-content: center;
    pointer-events: none;
  }

  .placeholder p {
    color: var(--text-secondary);
    font-size: 1.1em;
    margin: 0;
  }
</style>
