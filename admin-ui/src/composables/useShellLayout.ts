import { ref, watch } from 'vue'

const STORAGE_KEY = 'adminSidebarCollapsed'

function readCollapsed() {
  try {
    return localStorage.getItem(STORAGE_KEY) === 'true'
  } catch {
    return false
  }
}

const collapsed = ref(readCollapsed())
const drawerOpen = ref(false)

watch(
  collapsed,
  (value) => {
    try {
      localStorage.setItem(STORAGE_KEY, String(value))
    } catch {
      // 存储不可用不影响当前会话。
    }
  },
  { flush: 'sync' },
)

export function useShellLayout() {
  return {
    collapsed,
    drawerOpen,
    toggleCollapsed() {
      collapsed.value = !collapsed.value
    },
    openDrawer() {
      drawerOpen.value = true
    },
    closeDrawer() {
      drawerOpen.value = false
    },
  }
}
