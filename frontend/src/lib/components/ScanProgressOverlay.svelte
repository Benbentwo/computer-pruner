<script lang="ts">
  import { scanProgress, isScanning } from '../stores'
  import { formatBytes, formatPath, formatDuration } from '../utils/format'
  import { cancelScan } from '../utils/wailsBindings'

  async function handleCancel() {
    await cancelScan()
    isScanning.set(false)
  }
</script>

<div class="overlay">
  <div class="progress-modal">
    <!-- Animated Spinner -->
    <div class="spinner-container">
      <div class="spinner" />
    </div>

    <!-- Progress Info -->
    <h2>Scanning…</h2>

    {#if $scanProgress}
      <div class="progress-info">
        <div class="progress-stat">
          <span class="stat-label">Items Scanned:</span>
          <span class="stat-value">{$scanProgress.scannedItems.toLocaleString()}</span>
        </div>

        <div class="progress-stat">
          <span class="stat-label">Total Size:</span>
          <span class="stat-value">{formatBytes($scanProgress.totalSize)}</span>
        </div>

        <div class="progress-stat">
          <span class="stat-label">Current Path:</span>
          <span class="stat-value path">
            {formatPath($scanProgress.currentPath, 50)}
          </span>
        </div>

        <div class="progress-stat">
          <span class="stat-label">Elapsed Time:</span>
          <span class="stat-value">{formatDuration($scanProgress.elapsedMs)}</span>
        </div>

        <!-- Progress Bar -->
        <div class="progress-bar-container">
          <div class="progress-bar" />
        </div>
      </div>
    {:else}
      <p class="text-secondary text-sm">Preparing scan…</p>
    {/if}

    <!-- Cancel Button -->
    <button class="cancel-btn secondary" on:click={handleCancel}>
      Cancel Scan
    </button>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: rgba(0, 0, 0, 0.75);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    backdrop-filter: blur(4px);
    animation: fadeIn 200ms ease;
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  .progress-modal {
    background-color: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 2em;
    max-width: 420px;
    width: 90%;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1.25em;
    box-shadow: 0 16px 48px rgba(0, 0, 0, 0.6);
    animation: slideUp 300ms ease;
  }

  @keyframes slideUp {
    from { transform: translateY(20px); opacity: 0; }
    to { transform: translateY(0); opacity: 1; }
  }

  .spinner-container {
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .spinner {
    width: 44px;
    height: 44px;
    border: 3px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  h2 {
    margin: 0;
    text-align: center;
    color: var(--text-primary);
    font-size: 1.2em;
  }

  .progress-info {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 0.75em;
  }

  .progress-stat {
    display: flex;
    flex-direction: column;
    gap: 0.15em;
  }

  .stat-label {
    font-size: 0.75em;
    color: var(--text-tertiary);
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .stat-value {
    font-size: 0.95em;
    color: var(--text-primary);
    font-weight: 600;
  }

  .stat-value.path {
    font-family: 'SF Mono', 'Monaco', 'Courier New', monospace;
    font-size: 0.8em;
    word-break: break-all;
    font-weight: 500;
    color: var(--text-secondary);
  }

  .progress-bar-container {
    width: 100%;
    height: 3px;
    background-color: var(--bg-tertiary);
    border-radius: 2px;
    overflow: hidden;
    margin-top: 0.25em;
  }

  .progress-bar {
    height: 100%;
    background: linear-gradient(90deg, var(--accent), var(--accent-hover));
    animation: indeterminate 1.5s ease-in-out infinite;
    width: 40%;
    border-radius: 2px;
  }

  @keyframes indeterminate {
    0% { transform: translateX(-100%); }
    100% { transform: translateX(350%); }
  }

  .cancel-btn {
    align-self: center;
    padding: 0.6em 1.5em;
    margin-top: 0.25em;
  }
</style>
