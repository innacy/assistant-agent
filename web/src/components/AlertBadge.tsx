import clsx from 'clsx'
import type { AlertPriority, AlertStatus, AlertType } from '../lib/types'
import { PRIORITY_COLORS, STATUS_LABELS, TYPE_COLORS, TYPE_LABELS } from '../lib/constants'

interface AlertBadgeProps {
  kind: 'type' | 'status' | 'priority'
  value: AlertType | AlertStatus | AlertPriority
  className?: string
}

export function AlertBadge({ kind, value, className }: AlertBadgeProps) {
  if (kind === 'type') {
    const type = value as AlertType
    const color = TYPE_COLORS[type]
    return (
      <span
        className={clsx(
          'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium capitalize',
          className,
        )}
        style={{
          backgroundColor: `${color}22`,
          color,
          border: `1px solid ${color}44`,
        }}
      >
        {TYPE_LABELS[type]}
      </span>
    )
  }

  if (kind === 'status') {
    const status = value as AlertStatus
    const styles: Record<AlertStatus, string> = {
      upcoming: 'bg-blue-500/15 text-blue-300 border-blue-500/30',
      due_today: 'bg-amber-500/15 text-amber-300 border-amber-500/30',
      missed: 'bg-red-500/15 text-red-300 border-red-500/30',
      snoozed: 'bg-purple-500/15 text-purple-300 border-purple-500/30',
      acknowledged: 'bg-zinc-500/15 text-zinc-400 border-zinc-500/30',
    }
    return (
      <span
        className={clsx(
          'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium border capitalize',
          styles[status],
          className,
        )}
      >
        {STATUS_LABELS[status]}
      </span>
    )
  }

  const priority = value as AlertPriority
  return (
    <span
      className={clsx(
        'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium capitalize',
        PRIORITY_COLORS[priority],
        className,
      )}
    >
      {priority}
    </span>
  )
}
