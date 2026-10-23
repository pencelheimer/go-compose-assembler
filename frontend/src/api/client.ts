import type { ApiErrorBody } from './types'

/** A non-2xx API response, carrying the structured body from internal/web. */
export class ApiError extends Error {
  readonly status: number
  readonly body: ApiErrorBody

  constructor(status: number, body: ApiErrorBody) {
    super(body.message)
    this.name = 'ApiError'
    this.status = status
    this.body = body
  }
}

async function request<T>(path: string, init: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: { Accept: 'application/json', ...init.headers },
  })
  const isJson = res.headers.get('Content-Type')?.includes('application/json')
  if (res.ok) {
    if (!isJson) throw new Error(`Unexpected response from ${path}`)
    return (await res.json()) as T
  }
  if (isJson) throw new ApiError(res.status, (await res.json()) as ApiErrorBody)
  throw new ApiError(res.status, {
    kind: 'internal',
    message: `${res.status} ${res.statusText || 'request failed'}`,
    details: [],
  })
}

export const api = {
  get: <T>(path: string, signal?: AbortSignal) => request<T>(path, { signal }),
  post: <T>(path: string, body: unknown, signal?: AbortSignal) =>
    request<T>(path, {
      method: 'POST',
      body: JSON.stringify(body),
      headers: { 'Content-Type': 'application/json' },
      signal,
    }),
}
