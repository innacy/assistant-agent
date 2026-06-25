export type AlertType = 'birthday' | 'subscription' | 'payment' | 'task' | 'event'
export type AlertStatus = 'upcoming' | 'due_today' | 'missed' | 'snoozed' | 'acknowledged'
export type AlertPriority = 'low' | 'medium' | 'high'
export type AlertSource = 'gmail' | 'calendar' | 'tasks' | 'contacts' | 'manual'
export type Recurrence = 'none' | 'weekly' | 'monthly' | 'yearly'

export interface Alert {
  id: string
  user_id: string
  type: AlertType
  title: string
  description?: string
  due_date: string
  status: AlertStatus
  source: AlertSource
  priority: AlertPriority
  amount?: number
  currency?: string
  recurrence: Recurrence
  snoozed_until?: string
  acknowledged_at?: string
  tags?: string[]
  created_at: string
  updated_at: string
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  limit: number
  offset: number
  has_more: boolean
}

export interface AlertFilter {
  type?: string
  status?: string
  from?: string
  to?: string
  limit?: number
  offset?: number
}

export interface CreateAlertInput {
  type: AlertType
  title: string
  description?: string
  due_date: string
  recurrence?: Recurrence
  amount?: number
  priority?: AlertPriority
  tags?: string[]
}

export interface Settings {
  id?: string
  user_id?: string
  windows: Record<string, number>
  ttl: Record<string, number>
  poll_interval: string
  timezone: string
  initial_lookback: string
}

export interface SyncState {
  id?: string
  user_id?: string
  source: string
  last_sync_at?: string
  total_processed: number
  last_error?: string
  status: 'idle' | 'syncing' | 'error'
}

export interface SyncStatusResponse {
  data: SyncState[]
}
