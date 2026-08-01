<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { login } from '../api/auth'
import { useSession } from '../composables/useSession'

const router = useRouter()
const { setSession } = useSession()

const username = ref('')
const password = ref('')
const remember = ref(false)
const submitting = ref(false)
const error = ref('')

async function handleSubmit() {
  if (submitting.value) {
    return
  }
  error.value = ''
  submitting.value = true
  try {
    const session = await login(username.value, password.value, remember.value)
    setSession(session.username ?? username.value)
    // 登录后回到用户原本想访问的页面，没有则进仪表盘。
    const redirect = router.currentRoute.value.query.redirect
    await router.replace(typeof redirect === 'string' ? redirect : '/')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '登录失败，请稍后重试'
    password.value = ''
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <form class="login-card" @submit.prevent="handleSubmit">
      <h1 class="login-title">宠物后台</h1>
      <p class="login-subtitle">请使用管理员账号登录</p>

      <label class="field">
        <span class="field-label">账号</span>
        <input
          v-model="username"
          class="field-input"
          type="text"
          autocomplete="username"
          required
        />
      </label>

      <label class="field">
        <span class="field-label">密码</span>
        <input
          v-model="password"
          class="field-input"
          type="password"
          autocomplete="current-password"
          required
        />
      </label>

      <label class="remember">
        <input v-model="remember" type="checkbox" />
        <span>记住我</span>
      </label>

      <p v-if="error" class="login-error" role="alert">{{ error }}</p>

      <button class="btn login-submit" type="submit" :disabled="submitting">
        {{ submitting ? '登录中…' : '登录' }}
      </button>
    </form>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100%;
  display: grid;
  place-items: center;
  padding: 24px;
}

.login-card {
  width: 100%;
  max-width: 360px;
  padding: 32px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 16px;
}

.login-title {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
}

.login-subtitle {
  margin: -8px 0 8px;
  font-size: 13px;
  color: var(--text-muted);
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field-label {
  font-size: 13px;
  color: var(--text-muted);
}

.field-input {
  padding: 10px 12px;
  font: inherit;
  color: var(--text-main);
  background-color: var(--bg-base);
  border: 1px solid var(--border-color);
  border-radius: 8px;
}

.field-input:focus {
  outline: none;
  border-color: var(--accent);
}

.remember {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text-muted);
  cursor: pointer;
}

.login-error {
  margin: 0;
  padding: 8px 12px;
  font-size: 13px;
  color: var(--accent-ink);
  background-color: var(--accent-soft);
  border-radius: 8px;
}

.login-submit {
  margin-top: 4px;
  justify-content: center;
}

.login-submit:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>
