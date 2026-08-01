<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { changePassword } from '../api/auth'
import { useSession } from '../composables/useSession'

const router = useRouter()
const { clearSession } = useSession()

const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const submitting = ref(false)
const error = ref('')
const notice = ref('')

async function handleSubmit() {
  if (submitting.value) {
    return
  }
  error.value = ''
  notice.value = ''
  submitting.value = true
  try {
    notice.value = await changePassword(
      currentPassword.value,
      newPassword.value,
      confirmPassword.value,
    )
    // 后端在改密成功后会销毁全部会话，因此这里必须回到登录页重新认证。
    clearSession()
    setTimeout(() => {
      router.replace({ name: 'login' })
    }, 1200)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '修改密码失败'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <section>
    <h1 class="page-title">系统设置</h1>

    <form class="card" @submit.prevent="handleSubmit">
      <h2 class="card-title">修改密码</h2>
      <p class="card-hint">密码长度至少 6 位，修改成功后需要重新登录。</p>

      <label class="field">
        <span class="field-label">当前密码</span>
        <input
          v-model="currentPassword"
          class="field-input"
          type="password"
          autocomplete="current-password"
          required
        />
      </label>

      <label class="field">
        <span class="field-label">新密码</span>
        <input
          v-model="newPassword"
          class="field-input"
          type="password"
          autocomplete="new-password"
          required
        />
      </label>

      <label class="field">
        <span class="field-label">确认新密码</span>
        <input
          v-model="confirmPassword"
          class="field-input"
          type="password"
          autocomplete="new-password"
          required
        />
      </label>

      <p v-if="error" class="form-message" role="alert">{{ error }}</p>
      <p v-else-if="notice" class="form-message" role="status">{{ notice }}</p>

      <button class="btn" type="submit" :disabled="submitting">
        {{ submitting ? '提交中…' : '修改密码' }}
      </button>
    </form>
  </section>
</template>

<style scoped>
.page-title {
  margin: 0 0 16px;
  font-size: 20px;
  font-weight: 600;
}

.card {
  max-width: 380px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 20px;
  background-color: var(--bg-base);
  border: 1px solid var(--border-color);
  border-radius: 12px;
}

.card-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}

.card-hint {
  margin: -6px 0 0;
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
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 8px;
}

.field-input:focus {
  outline: none;
  border-color: var(--accent);
}

.form-message {
  margin: 0;
  padding: 8px 12px;
  font-size: 13px;
  color: var(--accent-ink);
  background-color: var(--accent-soft);
  border-radius: 8px;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>
