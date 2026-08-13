<script setup lang="ts">
import { IconAlertTriangle, IconDatabaseOff, IconLoader2 } from '@tabler/icons-vue'

withDefaults(
  defineProps<{
    tone?: 'loading' | 'empty' | 'error'
    title: string
    description?: string
    actionLabel?: string
  }>(),
  { tone: 'empty', description: '', actionLabel: '' },
)

defineEmits<{ action: [] }>()
</script>

<template>
  <div class="ui-state" :class="`is-${tone}`" :role="tone === 'error' ? 'alert' : 'status'">
    <span class="ui-state-icon" aria-hidden="true">
      <IconLoader2 v-if="tone === 'loading'" class="spin" :size="24" />
      <IconAlertTriangle v-else-if="tone === 'error'" :size="24" />
      <IconDatabaseOff v-else :size="24" />
    </span>
    <strong>{{ title }}</strong>
    <p v-if="description">{{ description }}</p>
    <button v-if="actionLabel" class="btn btn-ghost" type="button" @click="$emit('action')">
      {{ actionLabel }}
    </button>
  </div>
</template>

<style scoped>
.ui-state {
  display: grid;
  justify-items: center;
  gap: 8px;
  min-height: 190px;
  align-content: center;
  padding: 32px;
  text-align: center;
  color: var(--text-muted);
  background: var(--bg-subtle);
  border: 1px dashed var(--border-strong);
  border-radius: var(--radius-card);
}
.ui-state strong { color: var(--text-main); font-size: 15px; }
.ui-state p { max-width: 440px; margin: 0; }
.ui-state-icon { display: grid; place-items: center; width: 48px; height: 48px; border-radius: 16px; background: var(--accent-soft); color: var(--accent); }
.ui-state.is-error .ui-state-icon { background: var(--danger-soft); color: var(--danger); }
.spin { animation: spin 800ms linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
