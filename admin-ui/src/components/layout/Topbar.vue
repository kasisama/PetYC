<script setup lang="ts">
import { useRouter } from 'vue-router'
import { logout } from '../../api/auth'
import { useSession } from '../../composables/useSession'
import { useTheme } from '../../composables/useTheme'

const router = useRouter()
const { theme, themes, setTheme } = useTheme()
const { username, clearSession } = useSession()

async function handleLogout() {
  try {
    await logout()
  } finally {
    // 即便请求失败也要清掉本地状态，避免停在一个已经无效的会话上。
    clearSession()
    router.replace({ name: 'login' })
  }
}
</script>

<template>
  <header class="topbar">
    <div class="theme-switch">
      <button
        v-for="item in themes"
        :key="item.id"
        class="btn-ghost theme-option"
        :class="{ active: theme === item.id }"
        type="button"
        @click="setTheme(item.id)"
      >
        {{ item.label }}
      </button>
    </div>

    <div class="account">
      <span class="account-name">{{ username || '管理员' }}</span>
      <button class="btn-ghost" type="button" @click="handleLogout">退出登录</button>
    </div>
  </header>
</template>

<style scoped>
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 20px;
}

.theme-switch {
  display: flex;
  gap: 4px;
}

.theme-option.active {
  color: var(--accent-ink);
  background-color: var(--accent-soft);
}

.account {
  display: flex;
  align-items: center;
  gap: 12px;
}

.account-name {
  font-size: 13px;
  color: var(--text-muted);
}
</style>
