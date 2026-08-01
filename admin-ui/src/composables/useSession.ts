import { ref } from 'vue'

// 会话凭据本身是服务端的 HttpOnly Cookie，前端只缓存用于展示的用户名，
// 以及一个「是否已登录」的判定结果，避免每次路由跳转都去探测一次接口。
const STORAGE_KEY = 'adminUsername'

function readStoredUsername(): string {
  try {
    return localStorage.getItem(STORAGE_KEY) ?? ''
  } catch {
    return ''
  }
}

const username = ref(readStoredUsername())
// null 表示尚未探测过，首次进入受保护路由时会请求一次 /auth/session。
const authenticated = ref<boolean | null>(null)

export function useSession() {
  return {
    username,
    authenticated,
    setSession(name: string) {
      username.value = name
      authenticated.value = true
      try {
        localStorage.setItem(STORAGE_KEY, name)
      } catch {
        // 写入失败只影响下次进入时的用户名回显。
      }
    },
    clearSession() {
      username.value = ''
      authenticated.value = false
      try {
        localStorage.removeItem(STORAGE_KEY)
      } catch {
        // 清理失败不影响本次登出。
      }
    },
  }
}
