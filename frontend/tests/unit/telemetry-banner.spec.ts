import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'

const { getConfig } = vi.hoisted(() => ({ getConfig: vi.fn() }))

vi.mock('@/services/api', () => ({
  default: { getConfig },
}))

import TelemetryBanner from '@/components/TelemetryBanner.vue'

describe('TelemetryBanner', () => {
  beforeEach(() => {
    localStorage.clear()
    getConfig.mockReset()
  })

  it('stays hidden when the effective telemetry setting is disabled', async () => {
    getConfig.mockResolvedValue({ success: true, data: { config: { telemetry: { enabled: false } } } })

    const wrapper = shallowMount(TelemetryBanner, {
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })
    await flushPromises()

    expect(wrapper.find('[data-test="telemetry-banner"]').exists()).toBe(false)
  })

  it('shows when the effective telemetry setting is enabled', async () => {
    getConfig.mockResolvedValue({ success: true, data: { config: { telemetry: { enabled: true } } } })

    const wrapper = shallowMount(TelemetryBanner, {
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })
    await flushPromises()

    expect(wrapper.find('[data-test="telemetry-banner"]').exists()).toBe(true)
  })
})
