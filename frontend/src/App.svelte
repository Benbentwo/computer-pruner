<script lang="ts">
  import { onMount } from 'svelte'
  import { theme, isScanning, scanTree, currentRoot } from '$lib/stores'
  import { initEventBridge } from '$lib/utils/eventBridge'
  import { startScan, startFolderScan } from '$lib/utils/wailsBindings'
  import DiskSelector from '$lib/components/DiskSelector.svelte'
  import SunburstChart from '$lib/components/SunburstChart.svelte'
  import Sidebar from '$lib/components/Sidebar.svelte'
  import BreadcrumbBar from '$lib/components/BreadcrumbBar.svelte'
  import CollectorPanel from '$lib/components/CollectorPanel.svelte'
  import ScanProgressOverlay from '$lib/components/ScanProgressOverlay.svelte'
  import Toast from '$lib/components/Toast.svelte'

  let currentView: 'selector' | 'analyzer' = 'selector'

  onMount(() => {
    initEventBridge()

    // Listen for scan completion to switch to analyzer view
    const unsub = scanTree.subscribe(tree => {
      if (tree) {
        currentView = 'analyzer'
      }
    })

    // Keyboard shortcuts
    function handleKeydown(e: KeyboardEvent) {
      if (e.key === 'Escape' && currentView === 'analyzer') {
        // Could go back one level here
      }
    }
    window.addEventListener('keydown', handleKeydown)

    return () => {
      unsub()
      window.removeEventListener('keydown', handleKeydown)
    }
  })

  async function handleScan(event: CustomEvent<{path: string, forceRescan: boolean}>) {
    const { path: mountPoint, forceRescan } = event.detail
    isScanning.set(true)

    try {
      // Determine if this is a volume mount point or a folder
      if (mountPoint === '/' || mountPoint.startsWith('/Volumes/')) {
        await startScan(mountPoint, forceRescan)
      } else {
        await startFolderScan(mountPoint, forceRescan)
      }
    } catch (e: any) {
      isScanning.set(false)
      console.error('Failed to start scan:', e)
    }
  }

  function goBackToSelector() {
    currentView = 'selector'
    scanTree.set(null)
    currentRoot.set(null)
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
          <button class="back-to-selector" on:click={goBackToSelector} title="Back to volume selector">
            ← Volumes
          </button>
        </div>
        <div class="top-bar-center">
          <BreadcrumbBar />
        </div>
        <div class="top-bar-right">
          <!-- Placeholder for future controls -->
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

  <!-- Toast Notifications -->
  <Toast />
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
    padding: 0.5em 1em;
    background-color: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-shrink: 0;
    gap: 1em;
    -webkit-app-region: drag;
  }

  .top-bar-left {
    flex: 0 0 auto;
    -webkit-app-region: no-drag;
  }

  .back-to-selector {
    padding: 0.35em 0.75em;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    border-radius: 6px;
    cursor: pointer;
    font-size: 0.8em;
    font-weight: 500;
    transition: all 150ms ease;
  }

  .back-to-selector:hover {
    background-color: var(--accent);
    border-color: var(--accent);
    color: white;
  }

  .top-bar-center {
    flex: 1;
    display: flex;
    justify-content: center;
    align-items: center;
    overflow: hidden;
    -webkit-app-region: no-drag;
  }

  .top-bar-right {
    flex: 0 0 auto;
    -webkit-app-region: no-drag;
  }

  .main-content {
    flex: 1;
    display: flex;
    overflow: hidden;
    background-color: var(--bg-primary);
  }

  .sidebar-left {
    flex: 0 0 280px;
    background-color: var(--bg-secondary);
    border-right: 1px solid var(--border);
    overflow: hidden;
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

  .bottom-panel {
    flex: 0 0 auto;
    background-color: var(--bg-secondary);
    border-top: 1px solid var(--border);
    overflow: hidden;
  }
</style>
