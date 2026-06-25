import type {
  Alert,
  AlertFilter,
  CreateAlertInput,
  PaginatedResponse,
  Settings,
  SyncStatusResponse,
} from './types'
import { TOKEN_STORAGE_KEY } from './constants'

class ApiError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

function getToken(): string | null {
  return localStorage.getItem(TOKEN_STORAGE_KEY)
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_STORAGE_KEY, token)
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_STORAGE_KEY)
}

function buildQuery(params: AlertFilter): string {
  const search = new URLSearchParams()
  const entries: [string, string | number | undefined][] = [
    ['type', params.type],
    ['status', params.status],
    ['from', params.from],
    ['to', params.to],
    ['limit', params.limit],
    ['offset', params.offset],
  ]
  for (const [key, value] of entries) {
    if (value !== undefined && value !== '') {
      search.set(key, String(value))
    }
  }
  const qs = search.toString()
  return qs ? `?${qs}` : ''
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  }
  if (token) {
    headers.Authorization = `Bearer ${token}`
  }

  const res = await fetch(path, { ...options, headers })

  if (!res.ok) {
    let message = res.statusText
    try {
      const body = await res.json()
      message = body.error ?? body.message ?? message
    } catch {
      // ignore parse errors
    }
    throw new ApiError(message, res.status)
  }

  if (res.status === 204) {
    return undefined as T
  }

  return res.json() as Promise<T>
}

export async function fetchAlerts(filter: AlertFilter = {}): Promise<PaginatedResponse<Alert>> {
  return request(`/api/alerts${buildQuery(filter)}`)
}

export async function fetchUpcoming(): Promise<PaginatedResponse<Alert>> {
  return request('/api/alerts/upcoming')
}

export async function fetchMissed(): Promise<PaginatedResponse<Alert>> {
  return request('/api/alerts/missed')
}

export async function fetchToday(): Promise<PaginatedResponse<Alert>> {
  return request('/api/alerts/today')
}

export async function fetchAlert(id: string): Promise<Alert> {
  return request(`/api/alerts/${id}`)
}

export async function createAlert(data: CreateAlertInput): Promise<Alert> {
  return request('/api/alerts', {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export async function updateAlert(id: string, data: Partial<CreateAlertInput>): Promise<Alert> {
  return request(`/api/alerts/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}

export async function deleteAlert(id: string): Promise<void> {
  await request(`/api/alerts/${id}`, { method: 'DELETE' })
}

export async function acknowledgeAlert(id: string): Promise<void> {
  await request(`/api/alerts/${id}/acknowledge`, { method: 'POST' })
}

export async function snoozeAlert(id: string, until: string): Promise<void> {
  await request(`/api/alerts/${id}/snooze`, {
    method: 'POST',
    body: JSON.stringify({ snooze_until: until }),
  })
}

export async function batchAcknowledge(ids: string[]): Promise<void> {
  await request('/api/alerts/batch/acknowledge', {
    method: 'POST',
    body: JSON.stringify({ ids }),
  })
}

export async function batchSnooze(ids: string[], until: string): Promise<void> {
  await request('/api/alerts/batch/snooze', {
    method: 'POST',
    body: JSON.stringify({ ids, snooze_until: until }),
  })
}

export async function fetchHistory(filter: AlertFilter = {}): Promise<PaginatedResponse<Alert>> {
  return request(`/api/history${buildQuery(filter)}`)
}

export async function fetchSyncStatus(): Promise<SyncStatusResponse> {
  return request('/api/sync/status')
}

export async function triggerSync(source?: string): Promise<{ ok: boolean; message: string }> {
  const qs = source ? `?source=${source}` : ''
  return request(`/api/sync/trigger${qs}`, { method: 'POST' })
}

export async function fetchSettings(): Promise<Settings> {
  return request('/api/settings')
}

export async function updateSettings(data: Partial<Settings>): Promise<void> {
  await request('/api/settings', {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}

export async function checkHealth(): Promise<{ status: string; service: string }> {
  return request('/health')
}

export { ApiError }
