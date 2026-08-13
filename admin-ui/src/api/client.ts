// 后端统一响应格式：HTTP 永远是 200，业务结果由 code 表达。
// 旧接口可能仍返回 {error}、{message,data} 或直接返回业务体，这里做兼容解包。
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

function messageFromBody(body: unknown, fallback: string): string {
  if (body && typeof body === 'object') {
    const o = body as Record<string, unknown>
    if (typeof o.error === 'string' && o.error) return o.error
    if (typeof o.msg === 'string' && o.msg) return o.msg
    if (typeof o.message === 'string' && o.message) return o.message
  }
  return fallback
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

  let payload: unknown
  try {
    payload = await response.json()
  } catch {
    if (!response.ok) {
      throw new ApiError(response.status, `请求失败（HTTP ${response.status}）`)
    }
    throw new ApiError(response.status, '响应不是合法 JSON')
  }

  if (!response.ok) {
    throw new ApiError(
      response.status,
      messageFromBody(payload, `请求失败（HTTP ${response.status}）`),
    )
  }

  // 标准 {code,msg,data}
  if (payload && typeof payload === 'object' && 'code' in payload) {
    const std = payload as ApiResponse<T>
    if (std.code !== 0) {
      throw new ApiError(std.code, std.msg || '请求失败')
    }
    return std.data
  }

  // 旧接口错误体：HTTP 200 + {error: "..."}（少数路径仍可能如此）
  if (
    payload &&
    typeof payload === 'object' &&
    'error' in payload &&
    typeof (payload as { error: unknown }).error === 'string' &&
    !('data' in payload) &&
    !('total' in payload)
  ) {
    throw new ApiError(1, (payload as { error: string }).error)
  }

  // 旧接口成功：整包返回（含 total/page/data 或 message/data）
  return payload as T
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  delete: <T>(path: string, body?: unknown) => request<T>('DELETE', path, body),
}
