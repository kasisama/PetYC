import { readonly, ref } from 'vue'

export type ToastTone = 'success' | 'error' | 'warning' | 'info'

export interface ToastMessage {
  id: number
  tone: ToastTone
  message: string
}

const toasts = ref<ToastMessage[]>([])
let nextId = 1
const timers = new Map<number, number>()

function dismiss(id: number) {
  const timer = timers.get(id)
  if (timer !== undefined) {
    window.clearTimeout(timer)
    timers.delete(id)
  }
  toasts.value = toasts.value.filter((toast) => toast.id !== id)
}

function push(tone: ToastTone, message: string, duration = 3600) {
  const id = nextId++
  toasts.value.push({ id, tone, message })
  if (duration > 0) {
    timers.set(id, window.setTimeout(() => dismiss(id), duration))
  }
  return id
}

export function useToast() {
  return {
    toasts: readonly(toasts),
    dismiss,
    success: (message: string, duration?: number) => push('success', message, duration),
    error: (message: string, duration?: number) => push('error', message, duration),
    warning: (message: string, duration?: number) => push('warning', message, duration),
    info: (message: string, duration?: number) => push('info', message, duration),
  }
}
