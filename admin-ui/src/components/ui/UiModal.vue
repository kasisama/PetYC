<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { IconX } from '@tabler/icons-vue'

const props = withDefaults(
  defineProps<{
    id?: string
    open: boolean
    title: string
    description?: string
    busy?: boolean
    size?: 'small' | 'medium' | 'large'
  }>(),
  {
    id: '',
    description: '',
    busy: false,
    size: 'medium',
  },
)

const emit = defineEmits<{ close: [] }>()
const panel = ref<HTMLElement | null>(null)

function close() {
  if (!props.busy) emit('close')
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    close()
    return
  }
  if (event.key !== 'Tab' || !panel.value) return

  const focusable = Array.from(
    panel.value.querySelectorAll<HTMLElement>(
      'button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])',
    ),
  )
  if (!focusable.length) return
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last?.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first?.focus()
  }
}

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    await nextTick()
    panel.value
      ?.querySelector<HTMLElement>('input, select, textarea, button:not([aria-label="关闭弹窗"])')
      ?.focus()
  },
  { immediate: true },
)
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="ui-modal-mask" @click.self="close" @keydown="onKeydown">
      <section
        ref="panel"
        class="ui-modal-panel"
        :class="`is-${size}`"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="`${id || 'ui-modal'}-title`"
      >
        <header class="ui-modal-head">
          <div>
            <h2 :id="`${id || 'ui-modal'}-title`">{{ title }}</h2>
            <p v-if="description">{{ description }}</p>
          </div>
          <button class="ui-icon-button" type="button" aria-label="关闭弹窗" :disabled="busy" @click="close">
            <IconX :size="18" />
          </button>
        </header>
        <div class="ui-modal-body"><slot /></div>
        <footer v-if="$slots.footer" class="ui-modal-footer"><slot name="footer" /></footer>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.ui-modal-mask {
  position: fixed;
  inset: 0;
  z-index: var(--z-modal);
  display: grid;
  place-items: center;
  padding: 24px;
  background: color-mix(in srgb, #140f18 52%, transparent);
  backdrop-filter: blur(6px);
}

.ui-modal-panel {
  width: min(100%, 640px);
  max-height: min(88dvh, 900px);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: var(--bg-surface);
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-dialog);
  box-shadow: var(--shadow-dialog);
  animation: modal-in 180ms var(--ease-out);
}

.ui-modal-panel.is-small { width: min(100%, 440px); }
.ui-modal-panel.is-large { width: min(100%, 920px); }

.ui-modal-head,
.ui-modal-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 20px 22px;
}

.ui-modal-head { border-bottom: 1px solid var(--border-color); }
.ui-modal-footer { justify-content: flex-end; border-top: 1px solid var(--border-color); }
.ui-modal-head h2 { margin: 0; font-size: 18px; line-height: 1.35; }
.ui-modal-head p { margin: 4px 0 0; color: var(--text-muted); font-size: 13px; }
.ui-modal-body { overflow: auto; padding: 22px; }

@keyframes modal-in {
  from { opacity: 0; transform: translateY(8px) scale(0.985); }
}

@media (max-width: 640px) {
  .ui-modal-mask { align-items: end; padding: 0; }
  .ui-modal-panel,
  .ui-modal-panel.is-small,
  .ui-modal-panel.is-large { width: 100%; max-height: 92dvh; border-radius: 20px 20px 0 0; }
}
</style>
