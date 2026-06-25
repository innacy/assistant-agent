import { useEffect, useState } from 'react'
import { formatDistanceToNow } from 'date-fns'
import {
  AlertCircle,
  CheckCircle2,
  Key,
  Loader2,
  RefreshCw,
  Save,
  Wifi,
  WifiOff,
} from 'lucide-react'
import clsx from 'clsx'
import { checkHealth, setToken } from '../lib/api'
import { TOKEN_STORAGE_KEY, TYPE_LABELS, ALERT_TYPES } from '../lib/constants'
import type { Settings } from '../lib/types'
import { useSettings, useSettingsMutations, useSyncStatus } from '../hooks/useAlerts'

const SOURCE_LABELS: Record<string, string> = {
  gmail: 'Gmail',
  calendar: 'Google Calendar',
  tasks: 'Google Tasks',
  contacts: 'Contacts',
}

function SyncStatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    idle: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30',
    syncing: 'bg-blue-500/15 text-blue-300 border-blue-500/30',
    error: 'bg-red-500/15 text-red-300 border-red-500/30',
  }
  return (
    <span
      className={clsx(
        'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium capitalize',
        styles[status] ?? 'bg-zinc-500/15 text-zinc-400',
      )}
    >
      {status}
    </span>
  )
}

export function SettingsPage() {
  const { data: settings, isLoading: settingsLoading } = useSettings()
  const { data: syncData, isLoading: syncLoading } = useSyncStatus()
  const { saveSettings, sync } = useSettingsMutations()

  const [token, setTokenValue] = useState(() => localStorage.getItem(TOKEN_STORAGE_KEY) ?? '')
  const [tokenSaved, setTokenSaved] = useState(false)
  const [healthOk, setHealthOk] = useState<boolean | null>(null)
  const [form, setForm] = useState<Partial<Settings>>({})

  useEffect(() => {
    if (settings) {
      setForm({
        windows: { ...settings.windows },
        ttl: { ...settings.ttl },
        poll_interval: settings.poll_interval,
        timezone: settings.timezone,
        initial_lookback: settings.initial_lookback,
      })
    }
  }, [settings])

  useEffect(() => {
    checkHealth()
      .then(() => setHealthOk(true))
      .catch(() => setHealthOk(false))
  }, [tokenSaved])

  const handleSaveToken = () => {
    setToken(token.trim())
    setTokenSaved(true)
    setTimeout(() => setTokenSaved(false), 2000)
  }

  const handleSaveSettings = (e: React.FormEvent) => {
    e.preventDefault()
    saveSettings.mutate(form)
  }

  const updateWindow = (type: string, value: number) => {
    setForm((f) => ({
      ...f,
      windows: { ...f.windows, [type]: value },
    }))
  }

  const updateTTL = (type: string, value: number) => {
    setForm((f) => ({
      ...f,
      ttl: { ...f.ttl, [type]: value },
    }))
  }

  if (settingsLoading) {
    return (
      <div className="flex justify-center py-24">
        <Loader2 className="size-8 animate-spin text-indigo-400" />
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-3xl space-y-8">
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-zinc-100">Settings</h1>
        <p className="mt-1 text-sm text-muted">Configure your assistant and sync preferences</p>
      </div>

      {/* Connection */}
      <section className="glass rounded-xl p-6">
        <div className="mb-4 flex items-center gap-2">
          <Key className="size-5 text-indigo-400" />
          <h2 className="text-lg font-semibold text-zinc-100">API Connection</h2>
        </div>

        <div className="mb-4 flex items-center gap-2 text-sm">
          {healthOk === null ? (
            <Loader2 className="size-4 animate-spin text-muted" />
          ) : healthOk ? (
            <>
              <Wifi className="size-4 text-emerald-400" />
              <span className="text-emerald-300">Backend connected</span>
            </>
          ) : (
            <>
              <WifiOff className="size-4 text-red-400" />
              <span className="text-red-300">Backend unreachable</span>
            </>
          )}
        </div>

        <label className="block">
          <span className="text-xs text-muted">API Token (from config.yaml server.api_token)</span>
          <div className="mt-1 flex gap-2">
            <input
              type="password"
              value={token}
              onChange={(e) => setTokenValue(e.target.value)}
              placeholder="Enter your API token"
              className="flex-1 rounded-lg border border-border bg-surface px-3 py-2 text-sm text-zinc-200 focus:border-indigo-500 focus:outline-none"
            />
            <button
              type="button"
              onClick={handleSaveToken}
              className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500"
            >
              {tokenSaved ? <CheckCircle2 className="size-4" /> : <Save className="size-4" />}
              {tokenSaved ? 'Saved' : 'Save'}
            </button>
          </div>
        </label>
      </section>

      {/* Sync Status */}
      <section className="glass rounded-xl p-6">
        <div className="mb-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <RefreshCw className="size-5 text-indigo-400" />
            <h2 className="text-lg font-semibold text-zinc-100">Sync Status</h2>
          </div>
          <button
            type="button"
            onClick={() => sync.mutate()}
            disabled={sync.isPending}
            className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:opacity-50"
          >
            {sync.isPending ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <RefreshCw className="size-4" />
            )}
            Sync Now
          </button>
        </div>

        {syncLoading ? (
          <div className="flex justify-center py-8">
            <Loader2 className="size-6 animate-spin text-indigo-400" />
          </div>
        ) : (syncData?.data ?? []).length === 0 ? (
          <p className="text-sm text-muted">No sync sources configured yet.</p>
        ) : (
          <div className="space-y-3">
            {(syncData?.data ?? []).map((state) => (
              <div
                key={state.source}
                className="rounded-lg border border-border bg-surface/50 p-4"
              >
                <div className="flex items-center justify-between">
                  <span className="font-medium text-zinc-200">
                    {SOURCE_LABELS[state.source] ?? state.source}
                  </span>
                  <SyncStatusBadge status={state.status} />
                </div>
                <div className="mt-2 grid gap-1 text-xs text-muted sm:grid-cols-2">
                  <span>
                    Last sync:{' '}
                    {state.last_sync_at
                      ? formatDistanceToNow(new Date(state.last_sync_at), { addSuffix: true })
                      : 'Never'}
                  </span>
                  <span>Processed: {state.total_processed.toLocaleString()}</span>
                </div>
                {state.last_error && (
                  <div className="mt-2 flex items-start gap-2 rounded-lg bg-red-500/10 p-2 text-xs text-red-300">
                    <AlertCircle className="mt-0.5 size-3.5 shrink-0" />
                    {state.last_error}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Alert Settings */}
      <section className="glass rounded-xl p-6">
        <h2 className="mb-4 text-lg font-semibold text-zinc-100">Alert Windows & TTL</h2>

        <form onSubmit={handleSaveSettings} className="space-y-6">
          <div>
            <h3 className="mb-3 text-sm font-medium text-zinc-300">Lookahead Windows (days)</h3>
            <div className="grid gap-3 sm:grid-cols-2">
              {ALERT_TYPES.map((type) => (
                <label key={type} className="flex items-center justify-between gap-3 rounded-lg border border-border bg-surface/50 px-3 py-2">
                  <span className="text-sm text-zinc-300">{TYPE_LABELS[type]}</span>
                  <input
                    type="number"
                    min="0"
                    max="365"
                    value={form.windows?.[type] ?? 0}
                    onChange={(e) => updateWindow(type, Number(e.target.value))}
                    className="w-16 rounded border border-border bg-surface px-2 py-1 text-sm text-zinc-200 text-center focus:border-indigo-500 focus:outline-none"
                  />
                </label>
              ))}
            </div>
          </div>

          <div>
            <h3 className="mb-3 text-sm font-medium text-zinc-300">TTL after due date (days)</h3>
            <div className="grid gap-3 sm:grid-cols-2">
              {ALERT_TYPES.map((type) => (
                <label key={type} className="flex items-center justify-between gap-3 rounded-lg border border-border bg-surface/50 px-3 py-2">
                  <span className="text-sm text-zinc-300">{TYPE_LABELS[type]}</span>
                  <input
                    type="number"
                    min="0"
                    max="365"
                    value={form.ttl?.[type] ?? 0}
                    onChange={(e) => updateTTL(type, Number(e.target.value))}
                    className="w-16 rounded border border-border bg-surface px-2 py-1 text-sm text-zinc-200 text-center focus:border-indigo-500 focus:outline-none"
                  />
                </label>
              ))}
            </div>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <label className="block">
              <span className="text-xs text-muted">Poll Interval</span>
              <input
                value={form.poll_interval ?? ''}
                onChange={(e) => setForm({ ...form, poll_interval: e.target.value })}
                placeholder="15m"
                className="mt-1 w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-zinc-200 focus:border-indigo-500 focus:outline-none"
              />
            </label>
            <label className="block">
              <span className="text-xs text-muted">Timezone</span>
              <input
                value={form.timezone ?? ''}
                onChange={(e) => setForm({ ...form, timezone: e.target.value })}
                placeholder="Asia/Kolkata"
                className="mt-1 w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-zinc-200 focus:border-indigo-500 focus:outline-none"
              />
            </label>
          </div>

          <div className="flex justify-end">
            <button
              type="submit"
              disabled={saveSettings.isPending}
              className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-5 py-2.5 text-sm font-medium text-white transition hover:bg-indigo-500 disabled:opacity-50"
            >
              {saveSettings.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <Save className="size-4" />
              )}
              Save Settings
            </button>
          </div>

          {saveSettings.isSuccess && (
            <p className="text-right text-xs text-emerald-400">Settings saved successfully.</p>
          )}
          {saveSettings.isError && (
            <p className="text-right text-xs text-red-400">Failed to save settings.</p>
          )}
        </form>
      </section>
    </div>
  )
}
