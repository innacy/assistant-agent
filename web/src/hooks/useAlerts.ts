import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { AlertFilter, CreateAlertInput, Settings } from '../lib/types'
import {
  acknowledgeAlert,
  batchAcknowledge,
  batchSnooze,
  createAlert,
  deleteAlert,
  fetchAlerts,
  fetchHistory,
  fetchMissed,
  fetchSettings,
  fetchSyncStatus,
  fetchToday,
  fetchUpcoming,
  snoozeAlert,
  triggerSync,
  updateAlert,
  updateSettings,
} from '../lib/api'
import { addDays, format } from 'date-fns'

export const alertKeys = {
  all: ['alerts'] as const,
  list: (filter: AlertFilter) => [...alertKeys.all, 'list', filter] as const,
  upcoming: () => [...alertKeys.all, 'upcoming'] as const,
  today: () => [...alertKeys.all, 'today'] as const,
  missed: () => [...alertKeys.all, 'missed'] as const,
  history: (filter: AlertFilter) => [...alertKeys.all, 'history', filter] as const,
}

export const settingsKeys = {
  all: ['settings'] as const,
  sync: () => [...settingsKeys.all, 'sync'] as const,
}

export function useAlerts(filter: AlertFilter = {}) {
  return useQuery({
    queryKey: alertKeys.list(filter),
    queryFn: () => fetchAlerts(filter),
  })
}

export function useUpcomingAlerts() {
  return useQuery({
    queryKey: alertKeys.upcoming(),
    queryFn: fetchUpcoming,
  })
}

export function useTodayAlerts() {
  return useQuery({
    queryKey: alertKeys.today(),
    queryFn: fetchToday,
  })
}

export function useMissedAlerts() {
  return useQuery({
    queryKey: alertKeys.missed(),
    queryFn: fetchMissed,
  })
}

export function useHistory(filter: AlertFilter = {}) {
  return useQuery({
    queryKey: alertKeys.history(filter),
    queryFn: () => fetchHistory(filter),
  })
}

export function useSettings() {
  return useQuery({
    queryKey: settingsKeys.all,
    queryFn: fetchSettings,
  })
}

export function useSyncStatus() {
  return useQuery({
    queryKey: settingsKeys.sync(),
    queryFn: fetchSyncStatus,
    refetchInterval: 30_000,
  })
}

export function useAlertMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: alertKeys.all })
  }

  const acknowledge = useMutation({
    mutationFn: acknowledgeAlert,
    onSuccess: invalidate,
  })

  const snooze = useMutation({
    mutationFn: ({ id, days }: { id: string; days: number }) =>
      snoozeAlert(id, format(addDays(new Date(), days), 'yyyy-MM-dd')),
    onSuccess: invalidate,
  })

  const batchAck = useMutation({
    mutationFn: batchAcknowledge,
    onSuccess: invalidate,
  })

  const batchSnz = useMutation({
    mutationFn: ({ ids, days }: { ids: string[]; days: number }) =>
      batchSnooze(ids, format(addDays(new Date(), days), 'yyyy-MM-dd')),
    onSuccess: invalidate,
  })

  const create = useMutation({
    mutationFn: (data: CreateAlertInput) => createAlert(data),
    onSuccess: invalidate,
  })

  const update = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<CreateAlertInput> }) =>
      updateAlert(id, data),
    onSuccess: invalidate,
  })

  const remove = useMutation({
    mutationFn: deleteAlert,
    onSuccess: invalidate,
  })

  return { acknowledge, snooze, batchAck, batchSnz, create, update, remove }
}

export function useSettingsMutations() {
  const queryClient = useQueryClient()

  const saveSettings = useMutation({
    mutationFn: (data: Partial<Settings>) => updateSettings(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: settingsKeys.all })
    },
  })

  const sync = useMutation({
    mutationFn: () => triggerSync(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: settingsKeys.sync() })
    },
  })

  return { saveSettings, sync }
}
