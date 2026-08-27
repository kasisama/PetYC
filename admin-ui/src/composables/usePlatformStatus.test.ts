import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../api/client'
import type { PlatformStatus } from '../api/ecosystem'
import { __resetPlatformStatusForTests, usePlatformStatus } from './usePlatformStatus'

const mocks = vi.hoisted(() => ({ getPlatformStatus: vi.fn() }))

vi.mock('../api/ecosystem', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/ecosystem')>()
  return { ...actual, getPlatformStatus: mocks.getPlatformStatus }
})

const Harness = defineComponent({
  setup() {
    return usePlatformStatus()
  },
  template: '<span>{{ state }}:{{ status?.qq_official?.connected }}</span>',
})

function platformStatus(connected: boolean): PlatformStatus {
  return { onebot: { connected: false }, qq_official: { connected, capabilities: {} } }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

beforeEach(() => {
  vi.useFakeTimers()
  mocks.getPlatformStatus.mockReset()
})

afterEach(() => {
  __resetPlatformStatusForTests()
  vi.useRealTimers()
})

describe('usePlatformStatus', () => {
  it('shares one request and refreshes all consumers every five seconds', async () => {
    mocks.getPlatformStatus.mockResolvedValueOnce(platformStatus(false)).mockResolvedValueOnce(platformStatus(true))
    const first = mount(Harness)
    const second = mount(Harness)
    await flushPromises()

    expect(mocks.getPlatformStatus).toHaveBeenCalledTimes(1)
    expect(first.text()).toContain('ready:false')
    expect(second.text()).toContain('ready:false')

    await vi.advanceTimersByTimeAsync(5_000)
    await flushPromises()
    expect(mocks.getPlatformStatus).toHaveBeenCalledTimes(2)
    expect(first.text()).toContain('ready:true')
    expect(second.text()).toContain('ready:true')

    first.unmount()
    second.unmount()
  })

  it('queues one fresh request behind an in-flight poll', async () => {
    const firstResponse = deferred<PlatformStatus>()
    mocks.getPlatformStatus.mockReturnValueOnce(firstResponse.promise).mockResolvedValueOnce(platformStatus(true))
    let refresh!: ReturnType<typeof usePlatformStatus>['refresh']
    const wrapper = mount(defineComponent({
      setup() {
        refresh = usePlatformStatus().refresh
        return () => null
      },
    }))

    const fresh = refresh({ fresh: true })
    expect(mocks.getPlatformStatus).toHaveBeenCalledTimes(1)
    firstResponse.resolve(platformStatus(false))
    await fresh
    await flushPromises()

    expect(mocks.getPlatformStatus).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('pauses while hidden and refreshes immediately when visible again', async () => {
    let hidden = false
    Object.defineProperty(document, 'hidden', { configurable: true, get: () => hidden })
    mocks.getPlatformStatus.mockResolvedValue(platformStatus(false))
    const wrapper = mount(Harness)
    await flushPromises()

    hidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(10_000)
    expect(mocks.getPlatformStatus).toHaveBeenCalledTimes(1)

    hidden = false
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(mocks.getPlatformStatus).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('marks transient failures unknown and retries without losing the last status', async () => {
    mocks.getPlatformStatus
      .mockResolvedValueOnce(platformStatus(true))
      .mockRejectedValueOnce(new Error('network down'))
      .mockResolvedValueOnce(platformStatus(false))
    const wrapper = mount(Harness)
    await flushPromises()
    expect(wrapper.text()).toContain('ready:true')

    await vi.advanceTimersByTimeAsync(5_000)
    await flushPromises()
    expect(wrapper.text()).toContain('unknown:true')

    await vi.advanceTimersByTimeAsync(5_000)
    await flushPromises()
    expect(wrapper.text()).toContain('ready:false')
    wrapper.unmount()
  })

  it('stops polling after an authorization failure and allows an explicit refresh to recover', async () => {
    mocks.getPlatformStatus.mockRejectedValue(new ApiError(401, 'expired'))
    let refresh!: ReturnType<typeof usePlatformStatus>['refresh']
    const wrapper = mount(defineComponent({
      setup() {
        const controls = usePlatformStatus()
        refresh = controls.refresh
        return controls
      },
      template: '<span>{{ state }}</span>',
    }))
    await flushPromises()
    expect(wrapper.text()).toContain('unknown')

    await vi.advanceTimersByTimeAsync(15_000)
    expect(mocks.getPlatformStatus).toHaveBeenCalledTimes(1)

    mocks.getPlatformStatus.mockReset().mockResolvedValue(platformStatus(true))
    await refresh({ fresh: true })
    await flushPromises()
    expect(wrapper.text()).toContain('ready')
    expect(mocks.getPlatformStatus).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(5_000)
    expect(mocks.getPlatformStatus).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('does not resurrect polling when the last consumer unmounts during a request', async () => {
    const response = deferred<PlatformStatus>()
    mocks.getPlatformStatus.mockReturnValue(response.promise)
    const wrapper = mount(Harness)
    expect(mocks.getPlatformStatus).toHaveBeenCalledTimes(1)
    wrapper.unmount()

    response.resolve(platformStatus(true))
    await flushPromises()
    await vi.advanceTimersByTimeAsync(10_000)
    expect(mocks.getPlatformStatus).toHaveBeenCalledTimes(1)
  })

  it('does not let an obsolete request clear a new subscription cycle timer', async () => {
    const obsoleteResponse = deferred<PlatformStatus>()
    mocks.getPlatformStatus
      .mockReturnValueOnce(obsoleteResponse.promise)
      .mockResolvedValueOnce(platformStatus(false))
      .mockResolvedValueOnce(platformStatus(true))

    const obsoleteWrapper = mount(Harness)
    obsoleteWrapper.unmount()
    const currentWrapper = mount(Harness)
    await flushPromises()
    expect(mocks.getPlatformStatus).toHaveBeenCalledTimes(2)

    obsoleteResponse.resolve(platformStatus(false))
    await flushPromises()
    await vi.advanceTimersByTimeAsync(5_000)
    await flushPromises()
    expect(mocks.getPlatformStatus).toHaveBeenCalledTimes(3)
    expect(currentWrapper.text()).toContain('ready:true')
    currentWrapper.unmount()
  })
})
