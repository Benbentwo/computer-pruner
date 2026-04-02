<script lang="ts">
  import { theme } from '$lib/stores'
  import DiskSelector from '$lib/components/DiskSelector.svelte'
  import SunburstChart from '$lib/components/SunburstChart.svelte'
  import Sidebar from '$lib/components/Sidebar.svelte'
  import BreadcrumbBar from '$lib/components/BreadcrumbBar.svelte'
  import CollectorPanel from '$lib/components/CollectorPanel.svelte'
  import ScanProgressOverlay from '$lib/components/ScanProgressOverlay.svelte'
  import { isScanning } from '$lib/stores'

  let currentView: 'selector' | 'analyzer' = 'selector'

  function handleScan(event: CustomEvent<string>) {
    currentView = 'analyzer'
    // TODO: Dispatch scan to backend with mount point
  }

  $: {
    document.documentElement.setAttribute('data-theme', $theme)
  }
</script>

<div class="app">
  {#if currentView === 'selector'}
    <div class="selector-container">
      <DiskSelector on:scan={handleScan} />
    </div>
  {:else}
    <div class="analyzer-layout">
      <!-- Top Bar -->
      <header class="top-bar">
        <div class="top-bar-left">
          <h1>Disk Analyzer</h1>
        </div>
        <div class="top-bar-right">
          <BreadcrumbBar />
        </div>
      </header>

      <!-- Main Content -->
      <main class="main-content">
        <!-- Left Sidebar -->
        <aside class="sidebar-left">
          <Sidebar />
        </aside>

        <!-- Center Chart -->
        <section class="chart-center">
          <SunburstChart />
        </section>

        <!-- Right Area (placeholder) -->
        <aside class="sidebar-right">
          <div class="placeholder-panel">
            <p class="text-secondary">Details panel</p>
          </div>
        </aside>
      </main>

      <!-- Bottom Collector Panel -->
      <footer class="bottom-panel">
        <CollectorPanel />
      </footer>
    </div>
  {/if}

  <!-- Scan Progress Overlay -->
  {#if $isScanning}
    <ScanProgressOverlay />
  {/if}
</div>

<style>
  .app {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    background-color: var(--bg-primary);
    color: var(--text-primary);
  }

  .selector-container {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 2em;
  }

  .analyzer-layout {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .top-bar {
    padding: 1em 1.5em;
    background-color: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-shrink: 0;
  }

  .top-bar-left {
    flex: 0 0 auto;
  }

  .top-bar-left h1 {
    margin: 0;
    font-size: 1.5em;
  }

  .top-bar-right {
    flex: 1;
    display: flex;
    justify-content: flex-end;
    align-items: center;
  }

  .main-content {
    flex: 1;
    display: flex;
    gap: 1px;
    overflow: hidden;
    background-color: var(--bg-primary);
  }

  .sidebar-left {
    flex: 0 0 280px;
    background-color: var(--bg-secondary);
    border-right: 1px solid var(--border);
    overflow-y: auto;
    display: flex;
    flex-direction: column;
  }

  .chart-center {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    background-color: var(--bg-primary);
  }

  .sidebar-right {
    flex: 0 0 280px;
    background-color: var(--bg-secondary);
    border-left: 1px solid var(--border);
    overflow-y: auto;
    display: flex;
    flex-direction: column;
  }

  .placeholder-panel {
    padding: 1.5em;
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-secondary);
  }

  .bottom-panel {
    flex: 0 0 auto;
    background-color: var(--bg-secondary);
    border-top: 1px solid var(--border);
    overflow: hidden;
  }
</style>
