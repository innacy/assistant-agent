import { Filter, RotateCcw } from 'lucide-react'
import type { AlertFilter } from '../lib/types'
import { ALERT_STATUSES, ALERT_TYPES, STATUS_LABELS, TYPE_LABELS } from '../lib/constants'

interface FilterBarProps {
  filter: AlertFilter
  onChange: (filter: AlertFilter) => void
  onReset: () => void
}

export function FilterBar({ filter, onChange, onReset }: FilterBarProps) {
  return (
    <div className="glass rounded-xl p-4">
      <div className="mb-3 flex items-center gap-2 text-sm font-medium text-zinc-300">
        <Filter className="size-4 text-indigo-400" />
        Filters
      </div>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <label className="flex flex-col gap-1">
          <span className="text-xs text-muted">Type</span>
          <select
            value={filter.type ?? ''}
            onChange={(e) => onChange({ ...filter, type: e.target.value || undefined, offset: 0 })}
            className="rounded-lg border border-border bg-surface px-3 py-2 text-sm text-zinc-200 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500/50"
          >
            <option value="">All types</option>
            {ALERT_TYPES.map((t) => (
              <option key={t} value={t}>
                {TYPE_LABELS[t]}
              </option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-xs text-muted">Status</span>
          <select
            value={filter.status ?? ''}
            onChange={(e) => onChange({ ...filter, status: e.target.value || undefined, offset: 0 })}
            className="rounded-lg border border-border bg-surface px-3 py-2 text-sm text-zinc-200 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500/50"
          >
            <option value="">All statuses</option>
            {ALERT_STATUSES.map((s) => (
              <option key={s} value={s}>
                {STATUS_LABELS[s]}
              </option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-xs text-muted">From</span>
          <input
            type="date"
            value={filter.from ?? ''}
            onChange={(e) => onChange({ ...filter, from: e.target.value || undefined, offset: 0 })}
            className="rounded-lg border border-border bg-surface px-3 py-2 text-sm text-zinc-200 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500/50"
          />
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-xs text-muted">To</span>
          <input
            type="date"
            value={filter.to ?? ''}
            onChange={(e) => onChange({ ...filter, to: e.target.value || undefined, offset: 0 })}
            className="rounded-lg border border-border bg-surface px-3 py-2 text-sm text-zinc-200 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500/50"
          />
        </label>

        <div className="flex items-end">
          <button
            type="button"
            onClick={onReset}
            className="inline-flex w-full items-center justify-center gap-2 rounded-lg border border-border px-3 py-2 text-sm text-muted transition hover:border-zinc-600 hover:text-zinc-200"
          >
            <RotateCcw className="size-4" />
            Reset
          </button>
        </div>
      </div>
    </div>
  )
}
