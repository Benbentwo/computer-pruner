<script lang="ts">
  import { volumes } from '../stores'
  import { formatBytes } from '../utils/format'
  import { createEventDispatcher } from 'svelte'

  const dispatch = createEventDispatcher<{ scan: string }>()

  function getUsagePercent(volume): number {
    return (volume.usedBytes / volume.totalBytes) * 100
  }

  function getStatusColor(percent: number): string {
    if (percent < 70) return 'green'
    if (percent < 90) return 'yellow'
    return 'red'
  }

  function handleScanClick(mountPoint: string) {
    dispatch('scan', mountPoint)
  }
</script>

<div class="selector">
  <div class="selector-header">
    <h2>Select a Volume to Analyze</h2>
    <p class="text-secondary">Choose a disk to visualize its usage</p>
  </div>

  {#if $volumes.length === 0}
    <div class="no-volumes">
      <p class="text-tertiary">Loading volumes...</p>
    </div>
  {:else}
    <div class="volumes-grid">
      {#each $volumes as volume (volume.mountPoint)}
        <button
          class="volume-card"
          on:click={() => handleScanClick(volume.mountPoint)}
        >
          <div class="card-header">
            <h3>{volume.name}</h3>
            <span class="fs-type text-secondary">{volume.fsType}</span>
          </div>

          <div class="mount-path text-tertiary text-sm">
            {volume.mountPoint}
          </div>

          <div class="capacity-info">
            <div class="usage-bar-container">
              <div
                class="usage-bar"
                class:green={getStatusColor(getUsagePercent(volume)) === 'green'}
                class:yellow={getStatusColor(getUsagePercent(volume)) === 'yellow'}
                class:red={getStatusColor(getUsagePercent(volume)) === 'red'}
                style="width: {getUsagePercent(volume)}%"
              />
            </div>

            <div class="capacity-text">
              <span class="text-sm">
                {formatBytes(volume.usedBytes)} / {formatBytes(volume.totalBytes)}
              </span>
              <span class="percent text-secondary">
                {Math.round(getUsagePercent(volume))}%
              </span>
            </div>
          </div>

          <div class="free-space text-tertiary text-sm">
            Free: {formatBytes(volume.freeBytes)}
          </div>
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .selector {
    width: 100%;
    max-width: 900px;
    display: flex;
    flex-direction: column;
    gap: 2em;
  }

  .selector-header {
    text-align: center;
  }

  .selector-header h2 {
    margin: 0 0 0.5em 0;
    color: var(--text-primary);
  }

  .selector-header p {
    margin: 0;
    color: var(--text-secondary);
  }

  .no-volumes {
    padding: 3em;
    text-align: center;
    color: var(--text-tertiary);
  }

  .volumes-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 1.5em;
  }

  .volume-card {
    padding: 1.5em;
    background-color: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    text-align: left;
    cursor: pointer;
    transition: all var(--transition);
    display: flex;
    flex-direction: column;
    gap: 0.75em;
  }

  .volume-card:hover {
    background-color: var(--bg-tertiary);
    border-color: var(--accent);
    transform: translateY(-2px);
    box-shadow: 0 4px 12px var(--shadow);
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    gap: 0.5em;
  }

  .card-header h3 {
    margin: 0;
    font-size: 1.1em;
    color: var(--text-primary);
  }

  .fs-type {
    font-size: 0.75em;
    font-weight: normal;
  }

  .mount-path {
    word-break: break-all;
  }

  .capacity-info {
    display: flex;
    flex-direction: column;
    gap: 0.5em;
  }

  .usage-bar-container {
    height: 8px;
    background-color: var(--bg-tertiary);
    border-radius: 4px;
    overflow: hidden;
  }

  .usage-bar {
    height: 100%;
    border-radius: 4px;
    transition: width 0.3s ease;
  }

  .usage-bar.green {
    background-color: var(--success);
  }

  .usage-bar.yellow {
    background-color: var(--warning);
  }

  .usage-bar.red {
    background-color: var(--danger);
  }

  .capacity-text {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .percent {
    font-weight: 600;
  }

  .free-space {
    padding-top: 0.25em;
    border-top: 1px solid var(--border);
  }
</style>
