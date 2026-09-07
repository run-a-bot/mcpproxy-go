import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'

const api = vi.hoisted(() => ({
  listAgentTokens: vi.fn(),
  getProfiles: vi.fn(),
  getActiveProfile: vi.fn(),
  revokeAgentToken: vi.fn(),
  regenerateAgentToken: vi.fn(),
  deleteAgentToken: vi.fn(),
  updateAgentToken: vi.fn(),
}))

vi.mock('@/services/api', () => ({ default: api }))

import AgentTokens from '@/views/AgentTokens.vue'

const baseToken = {
  token_prefix: 'mcp_agt_test',
  allowed_servers: ['*'],
  permissions: ['read'],
  created_at: '2026-01-01T00:00:00Z',
  last_used_at: null,
  access_profiles: [],
}

async function mountTokens() {
  const wrapper = mount(AgentTokens, { global: { plugins: [createPinia()] } })
  await new Promise(resolve => setTimeout(resolve, 110))
  await flushPromises()
  return wrapper
}

describe('AgentTokens managed token contract', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.getProfiles.mockResolvedValue({ success: true, data: { profiles: [] } })
    api.getActiveProfile.mockResolvedValue({ success: true, data: { active_profile: '' } })
    api.revokeAgentToken.mockResolvedValue({ success: true })
    api.regenerateAgentToken.mockResolvedValue({ success: true, data: { token: 'rotated' } })
    api.deleteAgentToken.mockResolvedValue({ success: true })
    api.updateAgentToken.mockResolvedValue({ success: true })
    HTMLDialogElement.prototype.showModal = vi.fn()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('renders zero-time managed tokens as non-expiring and active', async () => {
    api.listAgentTokens.mockResolvedValue({
      success: true,
      data: { tokens: [{ ...baseToken, name: 'runabot-u1010b5', expires_at: '0001-01-01T00:00:00Z', revoked: false }] },
    })

    const wrapper = await mountTokens()

    expect(wrapper.find('[data-test="managed-token-badge"]').text()).toBe('Managed')
    expect(wrapper.find('[data-test="managed-token-banner"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Never')
    expect(wrapper.text()).toContain('Active')
    expect(wrapper.find('[data-test="token-edit-runabot-u1010b5"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.find('[data-test="token-regenerate-runabot-u1010b5"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-test="token-revoke-runabot-u1010b5"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.find('[data-test="token-delete-runabot-u1010b5"]').exists()).toBe(false)
  })

  it('allows soft revoke but disables permanent deletion of its tombstone', async () => {
    api.listAgentTokens.mockResolvedValue({
      success: true,
      data: { tokens: [{ ...baseToken, name: 'runabot-u1010b5', expires_at: '0001-01-01T00:00:00Z', revoked: false }] },
    })
    vi.stubGlobal('confirm', vi.fn(() => true))
    const wrapper = await mountTokens()

    await wrapper.find('[data-test="token-revoke-runabot-u1010b5"]').trigger('click')
    await flushPromises()

    expect(confirm).toHaveBeenCalledWith(expect.stringContaining('will not automatically repair or recreate it'))
    expect(api.revokeAgentToken).toHaveBeenCalledWith('runabot-u1010b5')

    await wrapper.unmount()
    api.listAgentTokens.mockResolvedValue({
      success: true,
      data: { tokens: [{ ...baseToken, name: 'runabot-u1010b5', expires_at: '0001-01-01T00:00:00Z', revoked: true }] },
    })
    const revokedWrapper = await mountTokens()
    expect(revokedWrapper.find('[data-test="token-delete-runabot-u1010b5"]').attributes('disabled')).toBeDefined()
  })

  it('defaults ordinary token edits to preserving the exact expiry', async () => {
    api.listAgentTokens.mockResolvedValue({
      success: true,
      data: { tokens: [{ ...baseToken, name: 'custom-token', expires_at: '2026-12-01T00:00:00Z', revoked: false }] },
    })
    const wrapper = await mountTokens()

    await wrapper.find('[data-test="token-edit-custom-token"]').trigger('click')
    expect((wrapper.find('[data-test="edit-token-expiry"]').element as HTMLSelectElement).value).toBe('')
  })

  it('allows opening the edit dialog for managed tokens to assign profiles and permissions', async () => {
    api.listAgentTokens.mockResolvedValue({
      success: true,
      data: { tokens: [{ ...baseToken, name: 'runabot-u1010b5', expires_at: '0001-01-01T00:00:00Z', revoked: false }] },
    })
    api.getProfiles.mockResolvedValue({
      success: true,
      data: { profiles: [{ name: 'marketing', servers: ['hermes'], tool_count: 5 }] },
    })
    const wrapper = await mountTokens()

    await wrapper.find('[data-test="token-edit-runabot-u1010b5"]').trigger('click')
    expect(HTMLDialogElement.prototype.showModal).toHaveBeenCalled()

    const profileSelect = wrapper.find('[data-test="edit-token-access-profiles"]')
    expect(profileSelect.exists()).toBe(true)
  })
})
