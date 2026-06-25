import type { AlertPriority, AlertStatus, AlertType } from './types'

export const TYPE_COLORS: Record<AlertType, string> = {
  birthday: '#ec4899',
  subscription: '#3b82f6',
  payment: '#f97316',
  task: '#10b981',
  event: '#8b5cf6',
}

export const TYPE_LABELS: Record<AlertType, string> = {
  birthday: 'Birthday',
  subscription: 'Subscription',
  payment: 'Payment',
  task: 'Task',
  event: 'Event',
}

export const STATUS_LABELS: Record<AlertStatus, string> = {
  upcoming: 'Upcoming',
  due_today: 'Due Today',
  missed: 'Missed',
  snoozed: 'Snoozed',
  acknowledged: 'Acknowledged',
}

export const PRIORITY_COLORS: Record<AlertPriority, string> = {
  low: 'bg-zinc-700 text-zinc-300',
  medium: 'bg-amber-500/20 text-amber-300',
  high: 'bg-red-500/20 text-red-300',
}

export const ALERT_TYPES: AlertType[] = [
  'birthday',
  'subscription',
  'payment',
  'task',
  'event',
]

export const ALERT_STATUSES: AlertStatus[] = [
  'upcoming',
  'due_today',
  'missed',
  'snoozed',
  'acknowledged',
]

export const TOKEN_STORAGE_KEY = 'assistant_agent_token'
