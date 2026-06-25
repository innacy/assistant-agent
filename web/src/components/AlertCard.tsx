import clsx from 'clsx'
import { formatDistanceToNow, format, isPast } from 'date-fns'
import {
  BellOff,
  Calendar,
  Check,
  Clock,
  CreditCard,
  Gift,
  ListTodo,
  Loader2,
  PartyPopper,
} from 'lucide-react'
import type { Alert, AlertType } from '../lib/types'
import { TYPE_COLORS } from '../lib/constants'
import { AlertBadge } from './AlertBadge'

interface AlertCardProps {
  alert: Alert
  onAcknowledge?: (id: string) => void
  onSnooze?: (id: string, days: number) => void
  compact?: boolean
  isLoading?: boolean
  selected?: boolean
  onSelect?: (id: string) => void
  onClick?: (alert: Alert) => void
}

const TYPE_ICONS: Record<AlertType, typeof Gift> = {
  birthday: Gift,
  subscription: CreditCard,
  payment: CreditCard,
  task: ListTodo,
  event: PartyPopper,
}

function formatDueDate(dateStr: string): string {
  const date = new Date(dateStr)
  return format(date, 'MMM d, yyyy')
}

function formatRelative(dateStr: string): string {
  const date = new Date(dateStr)
  const prefix = isPast(date) ? '' : 'in '
  const suffix = isPast(date) ? ' ago' : ''
  return prefix + formatDistanceToNow(date, { addSuffix: false }) + suffix
}

export function AlertCard({
  alert,
  onAcknowledge,
  onSnooze,
  compact = false,
  isLoading = false,
  selected = false,
  onSelect,
  onClick,
}: AlertCardProps) {
  const Icon = TYPE_ICONS[alert.type]
  const color = TYPE_COLORS[alert.type]

  return (
    <article
      className={clsx(
        'group relative rounded-xl border bg-surface-raised/60 p-4 card-hover',
        selected ? 'border-indigo-500/60 ring-1 ring-indigo-500/30' : 'border-border',
        onClick && 'cursor-pointer',
      )}
      onClick={() => onClick?.(alert)}
    >
      <div
        className="absolute left-0 top-4 bottom-4 w-1 rounded-full"
        style={{ backgroundColor: color }}
      />

      <div className="flex items-start gap-3 pl-3">
        {onSelect && (
          <input
            type="checkbox"
            checked={selected}
            onChange={(e) => {
              e.stopPropagation()
              onSelect(alert.id)
            }}
            onClick={(e) => e.stopPropagation()}
            className="mt-1.5 size-4 rounded border-zinc-600 bg-surface text-indigo-500 focus:ring-indigo-500/50"
          />
        )}

        <div
          className="flex size-10 shrink-0 items-center justify-center rounded-lg"
          style={{ backgroundColor: `${color}18`, color }}
        >
          <Icon className="size-5" />
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="truncate font-medium text-zinc-100">{alert.title}</h3>
            <AlertBadge kind="priority" value={alert.priority} />
          </div>

          {!compact && alert.description && (
            <p className="mt-1 line-clamp-2 text-sm text-muted">{alert.description}</p>
          )}

          <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted">
            <span className="inline-flex items-center gap-1">
              <Calendar className="size-3" />
              {formatDueDate(alert.due_date)}
            </span>
            <span className="inline-flex items-center gap-1">
              <Clock className="size-3" />
              {formatRelative(alert.due_date)}
            </span>
            {alert.amount != null && (
              <span className="font-medium text-zinc-300">
                {alert.currency ?? 'INR'} {alert.amount.toLocaleString()}
              </span>
            )}
          </div>

          {!compact && (
            <div className="mt-2 flex flex-wrap gap-1.5">
              <AlertBadge kind="type" value={alert.type} />
              <AlertBadge kind="status" value={alert.status} />
            </div>
          )}
        </div>
      </div>

      {(onAcknowledge || onSnooze) && (
        <div
          className={clsx(
            'mt-3 flex flex-wrap gap-2 pl-3 opacity-100 transition-opacity',
            'md:opacity-0 md:group-hover:opacity-100 md:focus-within:opacity-100',
          )}
          onClick={(e) => e.stopPropagation()}
        >
          {onAcknowledge && (
            <button
              type="button"
              disabled={isLoading}
              onClick={() => onAcknowledge(alert.id)}
              className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-500/15 px-3 py-1.5 text-xs font-medium text-emerald-300 transition hover:bg-emerald-500/25 disabled:opacity-50"
            >
              {isLoading ? <Loader2 className="size-3 animate-spin" /> : <Check className="size-3" />}
              Done
            </button>
          )}
          {onSnooze && (
            <>
              <button
                type="button"
                disabled={isLoading}
                onClick={() => onSnooze(alert.id, 1)}
                className="inline-flex items-center gap-1.5 rounded-lg bg-purple-500/15 px-3 py-1.5 text-xs font-medium text-purple-300 transition hover:bg-purple-500/25 disabled:opacity-50"
              >
                <BellOff className="size-3" />
                1 day
              </button>
              <button
                type="button"
                disabled={isLoading}
                onClick={() => onSnooze(alert.id, 3)}
                className="inline-flex items-center gap-1.5 rounded-lg bg-purple-500/15 px-3 py-1.5 text-xs font-medium text-purple-300 transition hover:bg-purple-500/25 disabled:opacity-50"
              >
                <BellOff className="size-3" />
                3 days
              </button>
            </>
          )}
        </div>
      )}
    </article>
  )
}
