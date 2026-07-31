<script lang="ts">
  import { notifications } from '$lib/stores'
  import { onMount, onDestroy } from 'svelte'
  import type { Notification } from '$lib/stores'

  let visibleNotifications: Notification[] = []
  const timers = new Map<string, ReturnType<typeof setTimeout>>()

  const unsub = notifications.subscribe(items => {
    // Add new items that aren't already visible
    for (const item of items) {
      if (!visibleNotifications.find(n => n.id === item.id)) {
        visibleNotifications = [...visibleNotifications, item]
        // Auto-dismiss after 4 seconds
        const timer = setTimeout(() => dismiss(item.id), 4000)
        timers.set(item.id, timer)
      }
    }
  })

  function dismiss(id: string) {
    visibleNotifications = visibleNotifications.filter(n => n.id !== id)
    notifications.update(items => items.filter(n => n.id !== id))
    const timer = timers.get(id)
    if (timer) {
      clearTimeout(timer)
      timers.delete(id)
    }
  }

  onDestroy(() => {
    unsub()
    timers.forEach(t => clearTimeout(t))
  })
</script>

{#if visibleNotifications.length > 0}
  <div class="toast-container">
    {#each visibleNotifications as notification (notification.id)}
      <!--
        The whole toast is the dismiss target, so it has to be reachable and
        operable from the keyboard as well as the mouse: role + tabindex make it
        a real button to assistive tech, and the keydown handler gives Enter and
        Space the same effect as a click.
      -->
      <div
        class="toast toast-{notification.type}"
        role="button"
        tabindex="0"
        aria-label="Dismiss notification: {notification.message}"
        on:click={() => dismiss(notification.id)}
        on:keydown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            dismiss(notification.id)
          }
        }}
      >
        <span class="toast-icon">
          {#if notification.type === 'success'}✓
          {:else if notification.type === 'error'}✕
          {:else}ℹ
          {/if}
        </span>
        <span class="toast-message">{notification.message}</span>
      </div>
    {/each}
  </div>
{/if}

<style>
  .toast-container {
    position: fixed;
    bottom: 60px;
    right: 16px;
    display: flex;
    flex-direction: column-reverse;
    gap: 0.5em;
    z-index: 2000;
    pointer-events: none;
  }

  .toast {
    pointer-events: auto;
    display: flex;
    align-items: center;
    gap: 0.6em;
    padding: 0.6em 1em;
    border-radius: 8px;
    font-size: 0.85em;
    font-weight: 500;
    cursor: pointer;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
    animation: slideIn 250ms ease;
    max-width: 360px;
    backdrop-filter: blur(12px);
  }

  @keyframes slideIn {
    from { transform: translateX(100%); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }

  .toast-success {
    background-color: hsla(140, 60%, 35%, 0.95);
    color: hsl(140, 80%, 90%);
    border: 1px solid hsl(140, 50%, 45%);
  }

  .toast-error {
    background-color: hsla(0, 60%, 35%, 0.95);
    color: hsl(0, 80%, 90%);
    border: 1px solid hsl(0, 50%, 45%);
  }

  .toast-info {
    background-color: hsla(210, 60%, 35%, 0.95);
    color: hsl(210, 80%, 90%);
    border: 1px solid hsl(210, 50%, 45%);
  }

  .toast-icon {
    font-weight: 700;
    font-size: 1em;
    flex-shrink: 0;
  }

  .toast-message {
    flex: 1;
  }
</style>
