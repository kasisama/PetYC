import { onBeforeUnmount, onMounted, readonly, ref } from 'vue'
import { ApiError } from '../api/client'
import { getPlatformStatus, type PlatformStatus } from '../api/ecosystem'

export type PlatformStatusState = 'loading' | 'ready' | 'unknown'

const POLL_INTERVAL_MS = 5_000
const status = ref<PlatformStatus | null>(null)
const state = ref<PlatformStatusState>('loading')

let subscribers = 0
let timer: number | null = null
let inFlight: Promise<void> | null = null
let queuedFresh: Promise<void> | null = null
let epoch = 0
let authBlocked = false

function isVisible() {
  return typeof document === 'undefined' || !document.hidden
}

function clearTimer() {
  if (timer === null) return
  window.clearTimeout(timer)
  timer = null
}

function canPoll(requestEpoch = epoch) {
  return subscribers > 0 && requestEpoch === epoch && !authBlocked
}

function scheduleNext(requestEpoch = epoch) {
  if (!canPoll(requestEpoch) || !isVisible()) return
  clearTimer()
  timer = window.setTimeout(() => {
    timer = null
    void runCycle(false, requestEpoch).catch(() => undefined)
  }, POLL_INTERVAL_MS)
}

function startRequest(requestEpoch: number): Promise<void> {
  if (!canPoll(requestEpoch)) return Promise.resolve()
  if (status.value === null) state.value = 'loading'

  const request = getPlatformStatus()
    .then((nextStatus) => {
      if (!canPoll(requestEpoch)) return
      status.value = nextStatus
      state.value = 'ready'
    })
    .catch((error: unknown) => {
      if (canPoll(requestEpoch)) {
        state.value = 'unknown'
        if (error instanceof ApiError && (error.code === 401 || error.code === 403)) {
          authBlocked = true
          clearTimer()
        }
      }
      throw error
    })

  inFlight = request
  void request.finally(() => {
    if (inFlight === request) inFlight = null
  }).catch(() => undefined)
  return request
}

function requestStatus(fresh: boolean, requestEpoch: number): Promise<void> {
  if (!canPoll(requestEpoch)) return Promise.resolve()
  if (!inFlight) return startRequest(requestEpoch)
  if (!fresh) return inFlight
  if (queuedFresh) return queuedFresh

  const pending = inFlight
  const queued = pending
    .catch(() => undefined)
    .then(() => {
      if (!canPoll(requestEpoch)) return
      return startRequest(requestEpoch)
    })
  queuedFresh = queued
  void queued.finally(() => {
    if (queuedFresh === queued) queuedFresh = null
  }).catch(() => undefined)
  return queued
}

async function runCycle(fresh: boolean, requestEpoch = epoch) {
  if (!canPoll(requestEpoch)) return
  clearTimer()
  try {
    await requestStatus(fresh, requestEpoch)
  } finally {
    scheduleNext(requestEpoch)
  }
}

function handleVisibilityChange() {
  if (!isVisible()) {
    clearTimer()
    return
  }
  void runCycle(false).catch(() => undefined)
}

function subscribe() {
  subscribers += 1
  if (subscribers !== 1) return
  epoch += 1
  authBlocked = false
  document.addEventListener('visibilitychange', handleVisibilityChange)
  if (isVisible()) void runCycle(false).catch(() => undefined)
}

function unsubscribe() {
  subscribers = Math.max(0, subscribers - 1)
  if (subscribers !== 0) return
  epoch += 1
  clearTimer()
  inFlight = null
  queuedFresh = null
  document.removeEventListener('visibilitychange', handleVisibilityChange)
}

export function usePlatformStatus() {
  onMounted(subscribe)
  onBeforeUnmount(unsubscribe)

  return {
    status: readonly(status),
    state: readonly(state),
    refresh: (options: { fresh?: boolean } = {}) => {
      authBlocked = false
      return runCycle(Boolean(options.fresh))
    },
  }
}

export function __resetPlatformStatusForTests() {
  clearTimer()
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  subscribers = 0
  inFlight = null
  queuedFresh = null
  authBlocked = false
  epoch += 1
  status.value = null
  state.value = 'loading'
}
