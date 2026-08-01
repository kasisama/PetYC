// 后端统一响应格式：HTTP 永远是 200，业务结果由 code 表达。
export interface ApiResponse<T = unknown> {
  code: number
  msg: string
  data: T
}

export class ApiError extends Error {
  // tsconfig 开启了 erasableSyntaxOnly，不能使用构造函数参数属性，这里显式声明字段。
  readonly code: number

  constructor(code: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
  }
}

// 会话失效时的回调，由 router 注册，避免 api 层直接依赖路由实例。
let unauthorizedHandler: (() => void) | null = null

export function onUnauthorized(handler: () => void) {
  unauthorizedHandler = handler
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const response = await fetch(path, {
    method,
    // 后台使用服务端会话 Cookie，必须带上凭据。
    credentials: 'same-origin',
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })

  // 会话过期由中间件以 HTTP 401 返回，不走 {code,msg,data} 格式。
  if (response.status === 401) {
    unauthorizedHandler?.()
    throw new ApiError(401, '登录已失效，请重新登录')
  }

  if (!response.ok) {
    throw new ApiError(response.status, `请求失败（HTTP ${response.status}）`)
  }

  const payload = (await response.json()) as ApiResponse<T>
  if (payload.code !== 0) {
    throw new ApiError(payload.code, payload.msg || '请求失败')
  }
  return payload.data
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  delete: <T>(path: string) => request<T>('DELETE', path),
}
