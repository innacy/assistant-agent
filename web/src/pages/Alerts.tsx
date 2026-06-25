import { useState } from 'react'
import clsx from 'clsx'
import {
  ArrowDown,
  ArrowUp,
  BellOff,
  Check,
  ChevronLeft,
  ChevronRight,
  Loader2,
  Plus,
} from 'lucide-react'
import { AlertBadge } from '../components/AlertBadge'
import { AlertCard } from '../components/AlertCard'
import { FilterBar } from '../components/FilterBar'
import { Modal } from '../components/Modal'
import { useAlertMutations, useAlerts } from '../hooks/useAlerts'
import type { Alert, AlertFilter, AlertType, CreateAlertInput } from '../lib/types'
import { ALERT_TYPES, TYPE_LABELS } from '../lib/constants'
import { format } from 'date-fns'

const PAGE_SIZE = 20

type SortField = 'due_date' | 'title' | 'priority'
type SortDir = 'asc' | 'desc'

const PRIORITY_ORDER = { low: 0, medium: 1, high: 2 }

function CreateAlertForm({
  onSubmit,
  onCancel,
  isPending,
}: {
  onSubmit: (data: CreateAlertInput) => void
  onCancel: () => void
  isPending: boolean
}) {
  const [form, setForm] = useState<CreateAlertInput>({
    type: 'task',
    title: '',
    description: '',
    due_date: format(new Date(), 'yyyy-MM-dd'),
    priority: 'medium',
    recurrence: 'none',
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.title.trim()) return
    onSubmit(form)
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <label className="block">
        <span className="text-xs text-muted">Title</span>
        <input
          required
          value={form.title}
          onChange={(e) => setForm({ ...form, title: e.target.value })}
          className="mt-1 w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-zinc-200 focus:border-indigo-500 focus:outline-none"
          placeholder="Renew passport"
        />
      </label>

      <label className="block">
        <span className="text-xs text-muted">Description</span>
        <textarea
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
          rows={2}
          className="mt-1 w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-zinc-200 focus:border-indigo-500 focus:outline-none"
        />
      </label>

      <div className="grid gap-4 sm:grid-cols-2">
        <label className="block">
          <span className="text-xs text-muted">Type</span>
          <select
            value={form.type}
            onChange={(e) => setForm({ ...form, type: e.target.value as AlertType })}
            className="mt-1 w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-zinc-200 focus:border-indigo-500 focus:outline-none"
          >
            {ALERT_TYPES.map((t) => (
              <option key={t} value={t}>
                {TYPE_LABELS[t]}
              </option>
            ))}
          </select>
        </label>

        <label className="block">
          <span className="text-xs text-muted">Due date</span>
          <input
            type="date"
            required
            value={form.due_date}
            onChange={(e) => setForm({ ...form, due_date: e.target.value })}
            className="mt-1 w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-zinc-200 focus:border-indigo-500 focus:outline-none"
          />
        </label>

        <label className="block">
          <span className="text-xs text-muted">Priority</span>
          <select
            value={form.priority}
            onChange={(e) =>
              setForm({
                ...form,
                priority: e.target.value as CreateAlertInput['priority'],
              })
            }
            className="mt-1 w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-zinc-200 focus:border-indigo-500 focus:outline-none"
          >
            <option value="low">Low</option>
            <option value="medium">Medium</option>
            <option value="high">High</option>
          </select>
        </label>

        <label className="block">
          <span className="text-xs text-muted">Amount (optional)</span>
          <input
            type="number"
            min="0"
            step="0.01"
            value={form.amount ?? ''}
            onChange={(e) =>
              setForm({
                ...form,
                amount: e.target.value ? Number(e.target.value) : undefined,
              })
            }
            className="mt-1 w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-zinc-200 focus:border-indigo-500 focus:outline-none"
          />
        </label>
      </div>

      <div className="flex justify-end gap-2 pt-2">
        <button
          type="button"
          onClick={onCancel}
          className="rounded-lg px-4 py-2 text-sm text-muted transition hover:text-zinc-200"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={isPending}
          className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:opacity-50"
        >
          {isPending && <Loader2 className="size-4 animate-spin" />}
          Create Alert
        </button>
      </div>
    </form>
  )
}

function AlertDetailPanel({ alert, onClose }: { alert: Alert; onClose: () => void }) {
  return (
    <div className="mt-4 rounded-xl border border-border bg-surface p-4">
      <div className="flex items-start justify-between">
        <div>
          <h3 className="font-medium text-zinc-100">{alert.title}</h3>
          {alert.description && (
            <p className="mt-1 text-sm text-muted">{alert.description}</p>
          )}
        </div>
        <button type="button" onClick={onClose} className="text-xs text-muted hover:text-zinc-300">
          Close
        </button>
      </div>
      <div className="mt-3 flex flex-wrap gap-2">
        <AlertBadge kind="type" value={alert.type} />
        <AlertBadge kind="status" value={alert.status} />
        <AlertBadge kind="priority" value={alert.priority} />
      </div>
      <dl className="mt-4 grid gap-2 text-sm sm:grid-cols-2">
        <div>
          <dt className="text-xs text-muted">Source</dt>
          <dd className="capitalize text-zinc-300">{alert.source}</dd>
        </div>
        <div>
          <dt className="text-xs text-muted">Recurrence</dt>
          <dd className="capitalize text-zinc-300">{alert.recurrence}</dd>
        </div>
        {alert.tags && alert.tags.length > 0 && (
          <div className="sm:col-span-2">
            <dt className="text-xs text-muted">Tags</dt>
            <dd className="mt-1 flex flex-wrap gap-1">
              {alert.tags.map((tag) => (
                <span key={tag} className="rounded bg-surface-overlay px-2 py-0.5 text-xs text-zinc-400">
                  {tag}
                </span>
              ))}
            </dd>
          </div>
        )}
      </dl>
    </div>
  )
}

export function Alerts() {
  const [filter, setFilter] = useState<AlertFilter>({ limit: PAGE_SIZE, offset: 0 })
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [sortField, setSortField] = useState<SortField>('due_date')
  const [sortDir, setSortDir] = useState<SortDir>('asc')

  const { data, isLoading, isError } = useAlerts(filter)
  const { acknowledge, snooze, batchAck, batchSnz, create } = useAlertMutations()

  const alerts = [...(data?.data ?? [])].sort((a, b) => {
    let cmp = 0
    if (sortField === 'due_date') {
      cmp = new Date(a.due_date).getTime() - new Date(b.due_date).getTime()
    } else if (sortField === 'title') {
      cmp = a.title.localeCompare(b.title)
    } else {
      cmp = PRIORITY_ORDER[a.priority] - PRIORITY_ORDER[b.priority]
    }
    return sortDir === 'asc' ? cmp : -cmp
  })

  const toggleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortField(field)
      setSortDir('asc')
    }
  }

  const toggleSelect = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleAll = () => {
    if (selected.size === alerts.length) {
      setSelected(new Set())
    } else {
      setSelected(new Set(alerts.map((a) => a.id)))
    }
  }

  const page = Math.floor((filter.offset ?? 0) / PAGE_SIZE) + 1
  const totalPages = Math.max(1, Math.ceil((data?.total ?? 0) / PAGE_SIZE))

  const SortButton = ({ field, label }: { field: SortField; label: string }) => (
    <button
      type="button"
      onClick={() => toggleSort(field)}
      className={clsx(
        'inline-flex items-center gap-1 text-xs font-medium transition',
        sortField === field ? 'text-indigo-400' : 'text-muted hover:text-zinc-300',
      )}
    >
      {label}
      {sortField === field &&
        (sortDir === 'asc' ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />)}
    </button>
  )

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-zinc-100">All Alerts</h1>
          <p className="mt-1 text-sm text-muted">
            {data?.total ?? 0} total alerts
          </p>
        </div>
        <button
          type="button"
          onClick={() => setCreateOpen(true)}
          className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-indigo-500"
        >
          <Plus className="size-4" />
          Create Alert
        </button>
      </div>

      <FilterBar
        filter={filter}
        onChange={setFilter}
        onReset={() => setFilter({ limit: PAGE_SIZE, offset: 0 })}
      />

      {selected.size > 0 && (
        <div className="flex flex-wrap items-center gap-2 rounded-xl border border-indigo-500/30 bg-indigo-500/10 px-4 py-3">
          <span className="text-sm text-indigo-300">{selected.size} selected</span>
          <button
            type="button"
            onClick={() => batchAck.mutate([...selected])}
            disabled={batchAck.isPending}
            className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-500/20 px-3 py-1.5 text-xs font-medium text-emerald-300 hover:bg-emerald-500/30 disabled:opacity-50"
          >
            <Check className="size-3" />
            Acknowledge
          </button>
          <button
            type="button"
            onClick={() => batchSnz.mutate({ ids: [...selected], days: 1 })}
            disabled={batchSnz.isPending}
            className="inline-flex items-center gap-1.5 rounded-lg bg-purple-500/20 px-3 py-1.5 text-xs font-medium text-purple-300 hover:bg-purple-500/30 disabled:opacity-50"
          >
            <BellOff className="size-3" />
            Snooze 1 day
          </button>
          <button
            type="button"
            onClick={() => batchSnz.mutate({ ids: [...selected], days: 3 })}
            disabled={batchSnz.isPending}
            className="inline-flex items-center gap-1.5 rounded-lg bg-purple-500/20 px-3 py-1.5 text-xs font-medium text-purple-300 hover:bg-purple-500/30 disabled:opacity-50"
          >
            <BellOff className="size-3" />
            Snooze 3 days
          </button>
        </div>
      )}

      {/* Sort controls */}
      <div className="flex items-center gap-4">
        <span className="text-xs text-muted">Sort by:</span>
        <SortButton field="due_date" label="Due date" />
        <SortButton field="title" label="Title" />
        <SortButton field="priority" label="Priority" />
        {alerts.length > 0 && (
          <button
            type="button"
            onClick={toggleAll}
            className="ml-auto text-xs text-muted hover:text-zinc-300"
          >
            {selected.size === alerts.length ? 'Deselect all' : 'Select all'}
          </button>
        )}
      </div>

      {isLoading ? (
        <div className="flex justify-center py-16">
          <Loader2 className="size-8 animate-spin text-indigo-400" />
        </div>
      ) : isError ? (
        <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-6 text-center text-sm text-amber-200">
          Failed to load alerts. Check your API token in Settings.
        </div>
      ) : alerts.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border py-16">
          <p className="text-sm text-muted">No alerts match your filters.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {alerts.map((alert) => (
            <div key={alert.id}>
              <AlertCard
                alert={alert}
                selected={selected.has(alert.id)}
                onSelect={toggleSelect}
                onClick={(a) => setExpandedId(expandedId === a.id ? null : a.id)}
                onAcknowledge={(id) => acknowledge.mutate(id)}
                onSnooze={(id, days) => snooze.mutate({ id, days })}
              />
              {expandedId === alert.id && (
                <AlertDetailPanel alert={alert} onClose={() => setExpandedId(null)} />
              )}
            </div>
          ))}
        </div>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-4 pt-4">
          <button
            type="button"
            disabled={page <= 1}
            onClick={() => setFilter({ ...filter, offset: ((page - 2) * PAGE_SIZE) })}
            className="inline-flex items-center gap-1 rounded-lg border border-border px-3 py-1.5 text-sm text-muted transition hover:text-zinc-200 disabled:opacity-40"
          >
            <ChevronLeft className="size-4" />
            Previous
          </button>
          <span className="text-sm text-muted">
            Page {page} of {totalPages}
          </span>
          <button
            type="button"
            disabled={!data?.has_more}
            onClick={() => setFilter({ ...filter, offset: page * PAGE_SIZE })}
            className="inline-flex items-center gap-1 rounded-lg border border-border px-3 py-1.5 text-sm text-muted transition hover:text-zinc-200 disabled:opacity-40"
          >
            Next
            <ChevronRight className="size-4" />
          </button>
        </div>
      )}

      <Modal open={createOpen} onClose={() => setCreateOpen(false)} title="Create Alert">
        <CreateAlertForm
          isPending={create.isPending}
          onCancel={() => setCreateOpen(false)}
          onSubmit={(data) => {
            create.mutate(data, {
              onSuccess: () => setCreateOpen(false),
            })
          }}
        />
      </Modal>
    </div>
  )
}
