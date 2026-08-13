<script setup lang="ts">
import { IconAlertCircle, IconCheck, IconInfoCircle, IconX } from '@tabler/icons-vue'
import { useToast } from '../../composables/useToast'

const { toasts, dismiss } = useToast()
</script>

<template>
  <div class="toast-host" aria-live="polite" aria-atomic="false">
    <TransitionGroup name="toast-list">
      <article v-for="toast in toasts" :key="toast.id" class="toast-item" :class="`is-${toast.tone}`">
        <IconCheck v-if="toast.tone === 'success'" :size="18" aria-hidden="true" />
        <IconAlertCircle v-else-if="toast.tone === 'error' || toast.tone === 'warning'" :size="18" aria-hidden="true" />
        <IconInfoCircle v-else :size="18" aria-hidden="true" />
        <span>{{ toast.message }}</span>
        <button type="button" aria-label="关闭消息" @click="dismiss(toast.id)"><IconX :size="16" /></button>
      </article>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-host { position: fixed; top: 76px; right: 20px; z-index: var(--z-toast); display: grid; gap: 10px; width: min(380px, calc(100vw - 32px)); pointer-events: none; }
.toast-item { display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: 10px; padding: 13px 14px; border: 1px solid var(--border-strong); border-left: 3px solid var(--info); border-radius: 12px; background: var(--bg-surface); box-shadow: var(--shadow-popover); pointer-events: auto; }
.toast-item.is-success { border-left-color: var(--success); color: var(--success-strong); }
.toast-item.is-error { border-left-color: var(--danger); color: var(--danger); }
.toast-item.is-warning { border-left-color: var(--warning); color: var(--warning-strong); }
.toast-item span { color: var(--text-main); }
.toast-item button { display: grid; place-items: center; padding: 4px; border: 0; background: transparent; color: var(--text-muted); cursor: pointer; }
.toast-list-enter-active, .toast-list-leave-active { transition: opacity 180ms var(--ease-out), transform 180ms var(--ease-out); }
.toast-list-enter-from, .toast-list-leave-to { opacity: 0; transform: translateX(12px); }
@media (max-width: 640px) { .toast-host { top: 68px; right: 16px; } }
</style>
