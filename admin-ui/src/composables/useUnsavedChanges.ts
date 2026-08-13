import { onBeforeUnmount, onMounted, type Ref } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'

export const UNSAVED_MESSAGE = '当前页面有未保存的修改，确定要离开吗？'

export function confirmUnsavedNavigation(dirty: Ref<boolean>, confirm: (message: string) => boolean = window.confirm) {
  return !dirty.value || confirm(UNSAVED_MESSAGE)
}

export function useUnsavedChanges(dirty: Ref<boolean>) {
  const beforeUnload = (event: BeforeUnloadEvent) => {
    if (!dirty.value) return
    event.preventDefault()
    event.returnValue = ''
  }
  onMounted(() => window.addEventListener('beforeunload', beforeUnload))
  onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))
  onBeforeRouteLeave(() => confirmUnsavedNavigation(dirty))
}
