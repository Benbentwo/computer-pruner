<script lang="ts">
  import { scanProgress, isScanning } from '../stores'
  import { formatBytes, formatPath, formatDuration } from '../utils/format'

  function handleCancel() {
    // TODO: Dispatch cancel event to backend
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
    <h2>Scanning...</h2>

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
    background-color: rgba(0, 0, 0, 0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .progress-modal {
    background-color: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 2em;
    max-width: 400px;
    width: 90%;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1.5em;
    box-shadow: 0 10px 40px rgba(0, 0, 0, 0.5);
  }

  .spinner-container {
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .spinner {
    width: 48px;
    height: 48px;
    border: 3px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  h2 {
    margin: 0;
    text-align: center;
    color: var(--text-primary);
  }

  .progress-info {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 1em;
  }

  .progress-stat {
    display: flex;
    flex-direction: column;
    gap: 0.25em;
  }

  .stat-label {
    font-size: 0.8em;
    color: var(--text-secondary);
    font-weight: 500;
  }

  .stat-value {
    font-size: 1em;
    color: var(--text-primary);
    font-weight: 600;
  }

  .stat-value.path {
    font-family: 'Monaco', 'Courier New', monospace;
    font-size: 0.85em;
    word-break: break-all;
  }

  .progress-bar-container {
    width: 100%;
    height: 4px;
    background-color: var(--bg-tertiary);
    border-radius: 2px;
    overflow: hidden;
  }

  .progress-bar {
    height: 100%;
    background: linear-gradient(90deg, var(--accent), var(--accent-hover));
    animation: progress 2s ease-in-out infinite;
  }

  @keyframes progress {
    0% {
      width: 0%;
    }
    50% {
      width: 100%;
    }
    100% {
      width: 0%;
    }
  }

  .cancel-btn {
    align-self: center;
    padding: 0.65em 1.5em;
  }
</style>
