// 认证接口早于标准 {code,msg,data} 约定建立，仍使用 HTTP 状态码 + {error} 的形式，
// 因此单独封装，不复用 api/client.ts 的响应解包逻辑。
// 更重要的是：登录密码错误同样返回 401，若走通用拦截器会触发一次多余的跳转。

export interface SessionInfo {
  authenticated: boolean
  username?: string
}

export class AuthError extends Error {}

async function readError(response: Response, fallback: string): Promise<string> {
  try {
    const payload = (await response.json()) as { error?: string }
    return payload.error || fallback
  } catch {
    return fallback
  }
}

export async function login(
  username: string,
  password: string,
  remember: boolean,
): Promise<SessionInfo> {
  const response = await fetch('/api/admin/auth/login', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password, remember }),
  })
  if (!response.ok) {
    throw new AuthError(await readError(response, '登录失败，请稍后重试'))
  }
  return (await response.json()) as SessionInfo
}

export async function fetchSession(): Promise<SessionInfo> {
  const response = await fetch('/api/admin/auth/session', {
    credentials: 'same-origin',
  })
  if (!response.ok) {
    return { authenticated: false }
  }
  return (await response.json()) as SessionInfo
}

export async function logout(): Promise<void> {
  await fetch('/api/admin/auth/logout', {
    method: 'POST',
    credentials: 'same-origin',
  })
}

export async function changePassword(
  currentPassword: string,
  newPassword: string,
  confirmPassword: string,
): Promise<string> {
  const response = await fetch('/api/admin/auth/password', {
    method: 'PUT',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      current_password: currentPassword,
      new_password: newPassword,
      confirm_password: confirmPassword,
    }),
  })
  if (!response.ok) {
    throw new AuthError(await readError(response, '修改密码失败'))
  }
  const payload = (await response.json()) as { message?: string }
  return payload.message || '密码修改成功，请重新登录'
}
