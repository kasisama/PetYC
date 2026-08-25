import { onBeforeUnmount, onMounted, type Ref } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'

export const UNSAVED_MESSAGE = '当前页面有未保存的修改，确定要离开吗？'
export const UNSAVED_NAVIGATION_EVENT = 'admin:unsaved-navigation'

export type NavigationConfirmationRequest = (message: string) => Promise<boolean>

function requestNavigationConfirmation(message: string): Promise<boolean> {
	return new Promise((resolve) => {
		window.dispatchEvent(new CustomEvent(UNSAVED_NAVIGATION_EVENT, { detail: { message, resolve } }))
	})
}

export function confirmUnsavedNavigation(dirty: Ref<boolean>, request: NavigationConfirmationRequest = requestNavigationConfirmation) {
  return !dirty.value ? true : request(UNSAVED_MESSAGE)
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
