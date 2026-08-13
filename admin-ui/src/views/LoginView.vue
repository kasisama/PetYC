<script setup lang="ts">
import { IconEye, IconEyeOff, IconHeartFilled, IconLock, IconUser } from '@tabler/icons-vue'
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { login } from '../api/auth'
import { useSession } from '../composables/useSession'

const router = useRouter()
const route = useRoute()
const { setSession } = useSession()

const username = ref('')
const password = ref('')
const remember = ref(false)
const submitting = ref(false)
const error = ref('')
const showPassword = ref(false)
const sessionExpired = computed(() => route.query.reason === 'session-expired')

async function handleSubmit() {
  if (submitting.value) {
    return
  }
  error.value = ''
  submitting.value = true
  try {
    const session = await login(username.value, password.value, remember.value)
    setSession(session.username ?? username.value)
    const redirect = router.currentRoute.value.query.redirect
    await router.replace(typeof redirect === 'string' ? redirect : '/')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '账号或密码错误'
    password.value = ''
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <section class="login-shell">
      <aside class="login-aside">
        <span class="aside-kicker">QQ PET · OPERATIONS</span>
        <h2>把复杂配置留在后台，<br />把陪伴留给玩家。</h2>
        <p>集中管理宠物状态、群组开关、游戏配置与运营补偿。</p>
        <div class="ambient-orbit"><IconHeartFilled :size="48" /></div>
      </aside>
      <form class="login-card" @submit.prevent="handleSubmit">
      <div class="brand-row">
        <span class="brand-heart" aria-hidden="true"><IconHeartFilled :size="20" /></span>
        <div>
          <h1 class="login-title">宠物养成</h1>
          <p class="login-subtitle">米塔公寓运营台</p>
        </div>
      </div>
      <p class="login-deco">灯还亮着，它在等你</p>
      <p v-if="sessionExpired" class="session-notice" role="status">登录状态已过期，请重新验证管理员身份。</p>

      <label class="field">
        <span class="field-label">管理员账号</span>
        <span class="input-shell"><IconUser :size="18" aria-hidden="true" /><input
          v-model="username"
          class="field-input"
          type="text"
          autocomplete="username"
          required
        /></span>
      </label>

      <label class="field">
        <span class="field-label">管理员密码</span>
        <span class="input-shell"><IconLock :size="18" aria-hidden="true" /><input
          v-model="password"
          class="field-input"
          :type="showPassword ? 'text' : 'password'"
          autocomplete="current-password"
          required
        /><button class="password-toggle" type="button" :aria-label="showPassword ? '隐藏密码' : '显示密码'" @click="showPassword = !showPassword"><IconEyeOff v-if="showPassword" :size="18" /><IconEye v-else :size="18" /></button></span>
      </label>

      <label class="remember">
        <input v-model="remember" class="remember-input" type="checkbox" />
        <span class="remember-box" aria-hidden="true" />
        <span class="remember-text">记住我（15 天内免登录）</span>
      </label>

      <p v-if="error" class="login-error" role="alert">{{ error }}</p>

      <button class="btn login-submit" type="submit" :disabled="submitting">
        {{ submitting ? '登录中…' : '登录' }}
      </button>
      </form>
    </section>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100dvh;
  display: grid;
  place-items: center;
  padding: 24px;
  background:
    radial-gradient(ellipse 80% 60% at 50% 0%, var(--accent-soft), transparent 60%),
    linear-gradient(165deg, var(--bg-elevated) 0%, var(--bg-base) 55%);
}

.login-shell { width: min(960px, 100%); display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(360px, .8fr); overflow: hidden; border: 1px solid var(--border-strong); border-radius: 26px; background: var(--bg-surface); box-shadow: var(--shadow-dialog); }
.login-aside { position: relative; min-height: 560px; overflow: hidden; padding: 56px; background: color-mix(in srgb, var(--bg-elevated) 78%, var(--bg-surface)); }
.login-aside::before { content: ''; position: absolute; inset: 0; background: linear-gradient(135deg, transparent 30%, var(--accent-soft)), repeating-linear-gradient(90deg, transparent 0 79px, color-mix(in srgb, var(--accent) 7%, transparent) 80px); }
.login-aside > * { position: relative; z-index: 1; }
.aside-kicker { color: var(--accent); font-size: 11px; font-weight: 750; letter-spacing: .16em; }
.login-aside h2 { max-width: 520px; margin: 84px 0 18px; font-size: clamp(32px, 4vw, 54px); line-height: 1.14; letter-spacing: -.045em; }
.login-aside p { max-width: 430px; color: var(--text-muted); font-size: 15px; }
.ambient-orbit { position: absolute; right: -70px; bottom: -90px; display: grid; place-items: center; width: 290px; height: 290px; border: 1px solid color-mix(in srgb, var(--accent) 28%, transparent); border-radius: 50%; color: var(--accent); background: var(--accent-soft); box-shadow: inset 0 0 0 54px color-mix(in srgb, var(--accent) 5%, transparent); }

.login-card {
  width: 100%;
  padding: 48px 38px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  background-color: var(--bg-surface);
  border-left: 1px solid var(--border-color);
}

.brand-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.brand-heart {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 14px;
  background: var(--accent-soft);
  color: var(--accent);
  font-size: 18px;
}

.login-title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.login-subtitle {
  margin: 2px 0 0;
  font-size: 13px;
  color: var(--text-muted);
}

.login-deco {
  margin: 0 0 4px;
  font-size: 12px;
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
  width: 100%;
  padding: 10px 12px;
  font: inherit;
  color: var(--text-main);
  background-color: var(--bg-base);
  border: 0;
  background: transparent;
}

.input-shell { display: grid; grid-template-columns: auto 1fr auto; align-items: center; min-height: 46px; padding: 0 12px; border: 1px solid var(--border-color); border-radius: var(--radius-input); background: var(--bg-base); color: var(--text-muted); }
.input-shell:focus-within { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
.input-shell .field-input:focus { box-shadow: none; }
.password-toggle { display: grid; place-items: center; padding: 6px; border: 0; background: transparent; color: var(--text-muted); cursor: pointer; }
.session-notice { margin: 0; padding: 10px 12px; border-radius: 9px; background: var(--warning-soft); color: var(--warning-strong); font-size: 12px; }

.field-input:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.remember {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  color: var(--text-muted);
  cursor: pointer;
  user-select: none;
  width: fit-content;
}

.remember-input {
  position: absolute;
  opacity: 0;
  width: 1px;
  height: 1px;
  pointer-events: none;
}

.remember-box {
  position: relative;
  width: 18px;
  height: 18px;
  flex-shrink: 0;
  border-radius: 6px;
  border: 1.5px solid var(--border-color);
  background: var(--bg-base);
  box-shadow: inset 0 0 0 1px transparent;
  transition:
    background-color 0.15s ease,
    border-color 0.15s ease,
    box-shadow 0.15s ease;
}

.remember-box::after {
  content: '';
  position: absolute;
  left: 5px;
  top: 2px;
  width: 5px;
  height: 9px;
  border: solid var(--accent-ink);
  border-width: 0 2px 2px 0;
  transform: rotate(45deg) scale(0);
  opacity: 0;
  transition:
    transform 0.12s ease,
    opacity 0.12s ease;
}

.remember:hover .remember-box {
  border-color: color-mix(in srgb, var(--accent) 55%, var(--border-color));
}

.remember-input:focus-visible + .remember-box {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.remember-input:checked + .remember-box {
  background: var(--accent);
  border-color: var(--accent);
}

.remember-input:checked + .remember-box::after {
  opacity: 1;
  transform: rotate(45deg) scale(1);
}

.remember-text {
  line-height: 1.3;
}

.login-error {
  margin: 0;
  padding: 8px 12px;
  font-size: 13px;
  color: var(--danger);
  background-color: var(--danger-soft);
  border-radius: 8px;
}

.login-submit {
  margin-top: 4px;
  width: 100%;
}

.login-submit:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

@media (max-width: 820px) {
  .login-shell { grid-template-columns: 1fr; max-width: 430px; }
  .login-aside { display: none; }
  .login-card { border-left: 0; padding: 36px 26px; }
}
</style>
