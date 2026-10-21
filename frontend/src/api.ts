export type Profile = {
  id: number
  username: string
  role: 'teacher' | 'student'
  class_id: number
  class_name: string
  csrf_token: string
}

export type Material = {
  id: number
  original_filename: string
  uploader: string
  mime: string
  size_bytes: number
  created_at: string
  index_status: 'pending' | 'ready' | 'failed'
}

export type MaterialDetail = Material & { body: string }

export type RetrievalHit = {
  chunk_id: number
  material_id: number
  title: string
  chunk_index: number
  start_offset: number
  end_offset: number
  snippet: string
  score: number
  citation_number?: number
}

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
  }
}

const tokenKey = 'campusclaw_access_token'

export function getToken(): string | null {
  return sessionStorage.getItem(tokenKey)
}

export function setToken(token: string): void {
  sessionStorage.setItem(tokenKey, token)
}

export function clearToken(): void {
  sessionStorage.removeItem(tokenKey)
}

function requestInit(init?: RequestInit): RequestInit {
  const headers = new Headers(init?.headers)
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  return { ...init, credentials: 'omit', cache: 'no-store', headers }
}

export async function streamMessage(id: number, content: string, csrf: string, onEvent: (event: string, data: Record<string, unknown>) => void): Promise<void> {
  const response = await fetch(`/api/conversations/${id}/messages`, requestInit({
    method: 'POST', headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf }, body: JSON.stringify({ content }),
  }))
  if (!response.ok) {
    const payload = await response.json().catch(() => ({}))
    throw new ApiError(response.status, typeof payload.error === 'string' ? payload.error : '发送失败')
  }
  if (!response.body) throw new Error('浏览器无法读取流式回答')
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let pending = ''
  for (;;) {
    const { value, done } = await reader.read()
    pending += decoder.decode(value, { stream: !done })
    let end: number
    while ((end = pending.indexOf('\n\n')) >= 0) {
      const frame = pending.slice(0, end).replace(/\r/g, '')
      pending = pending.slice(end + 2)
      const event = frame.split('\n').find((line) => line.startsWith('event:'))?.slice(6).trim()
      const data = frame.split('\n').find((line) => line.startsWith('data:'))?.slice(5).trim()
      if (event && data) onEvent(event, JSON.parse(data) as Record<string, unknown>)
    }
    if (done) break
  }
}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, requestInit(init))
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new ApiError(response.status, typeof payload.error === 'string' ? payload.error : '请求失败，请稍后重试。')
  }
  return payload as T
}

export async function downloadMaterial(id: number, filename: string): Promise<void> {
  const response = await fetch(`/api/materials/${id}/file`, requestInit())
  if (!response.ok) {
    const payload = await response.json().catch(() => ({}))
    throw new ApiError(response.status, typeof payload.error === 'string' ? payload.error : '下载失败')
  }
  const objectURL = URL.createObjectURL(await response.blob())
  try {
    const link = document.createElement('a')
    link.href = objectURL
    link.download = filename
    document.body.append(link)
    link.click()
    link.remove()
  } finally {
    URL.revokeObjectURL(objectURL)
  }
}

export function messageFor(error: unknown): string {
  if (error instanceof ApiError) {
    switch (error.status) {
      case 400:
        if (error.message === 'only .txt and .md files are supported') return '文件扩展名不支持：请上传 .txt 或 .md 文件（.markdown 不受支持）。'
        if (error.message === 'non-empty UTF-8 text required') return '文件必须包含非空的 UTF-8 文本。'
        if (error.message === 'query must be 1-100 characters') return '检索词请输入 1–100 个字符。'
        return '文件或输入不符合要求，请检查后重试。'
      case 401: return '登录已失效，请重新登录。'
      case 403: return '当前账号没有此操作权限。'
      case 404: return '材料不存在或不属于你的班级。'
      case 413: return '文件超过允许大小。'
      case 503: return '服务暂时不可用，请稍后重试。'
      default: return error.message
    }
  }
  return '网络异常，请稍后重试。'
}
