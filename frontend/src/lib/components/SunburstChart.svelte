<script lang="ts">
  import { onMount } from 'svelte'
  import * as d3 from 'd3'
  import type { TreeNode } from '../types'
  import { currentRoot, hoveredNode, breadcrumbs, scanTree, collectorItems } from '../stores'
  import { getNodeColor } from '../utils/colorScale'
  import { formatBytes } from '../utils/format'

  let svgElement: SVGSVGElement
  let containerEl: HTMLDivElement
  let width = 600
  let height = 600

  $: radius = Math.min(width, height) / 2

  onMount(() => {
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        width = entry.contentRect.width
        height = entry.contentRect.height
      }
    })
    if (containerEl) ro.observe(containerEl)
    return () => ro.disconnect()
  })

  // Reactive: re-render when currentRoot or dimensions change
  $: if (svgElement && $currentRoot && radius > 50) {
    renderSunburst($currentRoot, radius)
  }

  function renderSunburst(root: TreeNode, r: number) {
    const svg = d3.select(svgElement)
    svg.selectAll('*').remove()

    // Build D3 hierarchy
    const hierarchy = d3.hierarchy(root, (d: any) => d.children)
      .sum((d: any) => d.isDir ? 0 : d.size)
      .sort((a, b) => (b.value ?? 0) - (a.value ?? 0))

    const partition = d3.partition<TreeNode>()
      .size([2 * Math.PI, r])
      .padding(0.5)

    const partitioned = partition(hierarchy)

    // Arc generator
    const innerRadius = r * 0.18
    const arc = d3.arc<d3.HierarchyRectangularNode<TreeNode>>()
      .startAngle(d => d.x0)
      .endAngle(d => d.x1)
      .innerRadius(d => Math.max(innerRadius, d.y0 * 0.85))
      .outerRadius(d => Math.max(innerRadius, d.y1 * 0.85 - 1))
      .padAngle(0.002)
      .padRadius(r / 2)

    const g = svg.append('g')
      .attr('transform', `translate(${width / 2}, ${height / 2})`)

    // Draw arcs
    const paths = g.selectAll('path')
      .data(partitioned.descendants().filter(d => d.depth > 0)) // Skip root
      .join('path')
      .attr('d', arc as any)
      .attr('fill', (d) => getNodeColor(d.data, d.depth))
      .attr('stroke', 'var(--bg-primary)')
      .attr('stroke-width', 0.5)
      .attr('cursor', 'pointer')
      .style('opacity', 1)
      .on('mouseenter', function(event, d) {
        d3.select(this).style('opacity', 0.8)
        hoveredNode.set(d.data)
      })
      .on('mouseleave', function() {
        d3.select(this).style('opacity', 1)
        hoveredNode.set(null)
      })
      .on('click', function(event, d) {
        if (d.data.isDir && d.data.children && d.data.children.length > 0) {
          drillDown(d.data)
        }
      })
      .on('contextmenu', function(event, d) {
        event.preventDefault()
        handleContextMenu(event, d.data)
      })

    // Tooltip title
    paths.append('title')
      .text(d => `${d.data.name}\n${formatBytes(d.value ?? 0)}`)

    // Center label
    const centerGroup = g.append('g')
      .attr('text-anchor', 'middle')
      .style('pointer-events', 'none')

    centerGroup.append('text')
      .attr('dy', '-0.3em')
      .attr('fill', 'var(--text-primary)')
      .attr('font-size', '1em')
      .attr('font-weight', '600')
      .text(truncateName(root.name, 16))

    centerGroup.append('text')
      .attr('dy', '1.1em')
      .attr('fill', 'var(--text-secondary)')
      .attr('font-size', '0.85em')
      .text(formatBytes(root.size))

    // Click center to go up
    g.append('circle')
      .attr('r', innerRadius)
      .attr('fill', 'transparent')
      .attr('cursor', 'pointer')
      .on('click', goUp)
  }

  function drillDown(node: TreeNode) {
    currentRoot.set(node)
    breadcrumbs.update(crumbs => {
      const idx = crumbs.findIndex(c => c.path === node.path)
      if (idx >= 0) {
        return crumbs.slice(0, idx + 1)
      }
      return [...crumbs, node]
    })
  }

  function goUp() {
    breadcrumbs.update(crumbs => {
      if (crumbs.length <= 1) {
        // Already at root of scan tree
        if ($scanTree) {
          currentRoot.set($scanTree)
          return [$scanTree]
        }
        return crumbs
      }
      const newCrumbs = crumbs.slice(0, -1)
      const parent = newCrumbs[newCrumbs.length - 1]
      currentRoot.set(parent)
      return newCrumbs
    })
  }

  function handleContextMenu(event: MouseEvent, node: TreeNode) {
    // Dispatch a custom event that App.svelte can handle
    const detail = { node, x: event.clientX, y: event.clientY }
    svgElement.dispatchEvent(new CustomEvent('nodecontextmenu', { detail, bubbles: true }))
  }

  function truncateName(name: string, max: number): string {
    if (name.length <= max) return name
    return name.substring(0, max - 1) + '…'
  }
</script>

<div class="chart-container" bind:this={containerEl}>
  <svg bind:this={svgElement} {width} {height} />

  {#if !$currentRoot}
    <div class="placeholder">
      <div class="placeholder-content">
        <div class="placeholder-icon">📊</div>
        <p>Scan a volume to visualize disk usage</p>
      </div>
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
    overflow: hidden;
  }

  svg {
    display: block;
  }

  .placeholder {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    pointer-events: none;
  }

  .placeholder-content {
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.75em;
  }

  .placeholder-icon {
    font-size: 3em;
    opacity: 0.4;
  }

  .placeholder p {
    color: var(--text-secondary);
    font-size: 1.1em;
    margin: 0;
  }
</style>
