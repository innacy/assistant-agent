import { addDays, isWithinInterval, startOfDay } from 'date-fns'
import {
  AlertTriangle,
  CalendarCheck,
  CalendarClock,
  Loader2,
  Sparkles,
} from 'lucide-react'
import { AlertCard } from '../components/AlertCard'
import { useAlertMutations, useMissedAlerts, useTodayAlerts, useUpcomingAlerts } from '../hooks/useAlerts'
import type { Alert } from '../lib/types'

function StatCard({
  label,
  value,
  icon: Icon,
  accent,
}: {
  label: string
  value: number
  icon: typeof Sparkles
  accent: string
}) {
  return (
    <div className="glass rounded-xl p-5 card-hover">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm text-muted">{label}</p>
          <p className="mt-1 text-3xl font-bold tracking-tight text-zinc-100">{value}</p>
        </div>
        <div
          className="flex size-11 items-center justify-center rounded-xl"
          style={{ backgroundColor: `${accent}18`, color: accent }}
        >
          <Icon className="size-5" />
        </div>
      </div>
    </div>
  )
}

function Section({
  title,
  icon: Icon,
  alerts,
  emptyMessage,
  onAcknowledge,
  onSnooze,
  loadingId,
}: {
  title: string
  icon: typeof Sparkles
  alerts: Alert[]
  emptyMessage: string
  onAcknowledge: (id: string) => void
  onSnooze: (id: string, days: number) => void
  loadingId: string | null
}) {
  return (
    <section>
      <div className="mb-4 flex items-center gap-2">
        <Icon className="size-5 text-indigo-400" />
        <h2 className="text-lg font-semibold text-zinc-100">{title}</h2>
        <span className="rounded-full bg-surface-overlay px-2 py-0.5 text-xs text-muted">
          {alerts.length}
        </span>
      </div>

      {alerts.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border py-12 text-center">
          <Sparkles className="mb-3 size-8 text-zinc-600" />
          <p className="text-sm text-muted">{emptyMessage}</p>
        </div>
      ) : (
        <div className="grid gap-3">
          {alerts.map((alert) => (
            <AlertCard
              key={alert.id}
              alert={alert}
              onAcknowledge={onAcknowledge}
              onSnooze={onSnooze}
              isLoading={loadingId === alert.id}
            />
          ))}
        </div>
      )}
    </section>
  )
}

function LoadingState() {
  return (
    <div className="flex flex-col items-center justify-center py-24">
      <Loader2 className="size-8 animate-spin text-indigo-400" />
      <p className="mt-3 text-sm text-muted">Loading your alerts...</p>
    </div>
  )
}

export function Dashboard() {
  const today = useTodayAlerts()
  const upcoming = useUpcomingAlerts()
  const missed = useMissedAlerts()
  const { acknowledge, snooze } = useAlertMutations()

  const isLoading = today.isLoading || upcoming.isLoading || missed.isLoading
  const isError = today.isError || upcoming.isError || missed.isError

  const loadingId =
    acknowledge.isPending
      ? (acknowledge.variables ?? null)
      : snooze.isPending
        ? (snooze.variables?.id ?? null)
        : null

  const now = startOfDay(new Date())
  const weekEnd = addDays(now, 7)

  const upcomingWeek = (upcoming.data?.data ?? []).filter((a) => {
    const due = new Date(a.due_date)
    return isWithinInterval(due, { start: now, end: weekEnd })
  })

  const todayCount = today.data?.total ?? 0
  const upcomingCount = upcoming.data?.total ?? 0
  const missedCount = missed.data?.total ?? 0

  if (isLoading) return <LoadingState />

  if (isError) {
    return (
      <div className="flex flex-col items-center justify-center py-24 text-center">
        <AlertTriangle className="mb-3 size-10 text-amber-400" />
        <h2 className="text-lg font-medium text-zinc-200">Could not load dashboard</h2>
        <p className="mt-1 max-w-md text-sm text-muted">
          Check that the backend is running and your API token is set in Settings.
        </p>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-5xl space-y-8">
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-zinc-100">Dashboard</h1>
        <p className="mt-1 text-sm text-muted">Your reminders at a glance</p>
      </div>

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Due Today" value={todayCount} icon={CalendarCheck} accent="#f59e0b" />
        <StatCard label="Upcoming" value={upcomingCount} icon={CalendarClock} accent="#6366f1" />
        <StatCard label="Missed" value={missedCount} icon={AlertTriangle} accent="#ef4444" />
      </div>

      <Section
        title="Due Today"
        icon={CalendarCheck}
        alerts={today.data?.data ?? []}
        emptyMessage="Nothing due today — enjoy your day!"
        onAcknowledge={(id) => acknowledge.mutate(id)}
        onSnooze={(id, days) => snooze.mutate({ id, days })}
        loadingId={loadingId}
      />

      <Section
        title="Upcoming (next 7 days)"
        icon={CalendarClock}
        alerts={upcomingWeek}
        emptyMessage="No upcoming alerts in the next week."
        onAcknowledge={(id) => acknowledge.mutate(id)}
        onSnooze={(id, days) => snooze.mutate({ id, days })}
        loadingId={loadingId}
      />

      <Section
        title="Recently Missed"
        icon={AlertTriangle}
        alerts={missed.data?.data ?? []}
        emptyMessage="You're all caught up — no missed alerts!"
        onAcknowledge={(id) => acknowledge.mutate(id)}
        onSnooze={(id, days) => snooze.mutate({ id, days })}
        loadingId={loadingId}
      />
    </div>
  )
}
