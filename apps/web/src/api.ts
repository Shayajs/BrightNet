import type { Activity, Graph, Summary } from './types'

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  const response = await fetch(path, { ...init, headers, credentials: 'include' })
  if (!response.ok) {
    let message = response.statusText
    try {
      const body = await response.json() as { error?: string }
      if (body.error) message = body.error
    } catch {
      /* le corps n'est pas du JSON */
    }
    throw new ApiError(response.status, message)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export const api = {
  setupStatus: () => request<{ needs_setup: boolean }>('/api/v1/setup/status'),
  setup: (username: string, password: string) => request('/api/v1/setup', { method: 'POST', body: JSON.stringify({ username, password }) }),
  login: (username: string, password: string) => request('/api/v1/login', { method: 'POST', body: JSON.stringify({ username, password }) }),
  logout: () => request('/api/v1/logout', { method: 'POST' }),
  me: () => request<{ username: string }>('/api/v1/me'),
  summary: () => request<Summary>('/api/v1/summary'),
  graph: (view: string) => request<Graph>(`/api/v1/graph?view=${encodeURIComponent(view)}`),
  events: () => request<{ events: Activity[] }>('/api/v1/events'),
  layout: (view: string) => request<Record<string, { x: number; y: number }>>(`/api/v1/layouts/${view}`),
  saveLayout: (view: string, positions: Record<string, { x: number; y: number }>) =>
    request(`/api/v1/layouts/${view}`, { method: 'PUT', body: JSON.stringify(positions) }),
  rename: (id: string, displayName: string) =>
    request(`/api/v1/entities/${id}`, { method: 'PATCH', body: JSON.stringify({ display_name: displayName }) }),
  loadFixture: () => request('/api/v1/fixtures/load', { method: 'POST' }),
  pairingCode: () => request<{ pairing_code: string; expires_at: string }>('/api/v1/pairing-codes', { method: 'POST' }),
}
