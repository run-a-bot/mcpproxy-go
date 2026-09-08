import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import Tools from '@/views/Tools.vue'
import api from '@/services/api'

vi.mock('@/services/api', () => ({
  default: {
    getGlobalTools: vi.fn(),
    setToolEnabled: vi.fn(),
    approveTools: vi.fn(),
    blockTools: vi.fn(),
    overrideTools: vi.fn(),
    resetToolOverrides: vi.fn(),
  },
}))

const globalStubs = { CollapsibleHintsPanel: { template: '<div />' } }

const TOOLS = [
  {
    name: 'get_issue',
    server_name: 'jira',
    approval_status: 'approved',
    enabled: true,
    description: 'Get issue details',
    annotations: { readOnlyHint: true },
    is_custom_override: false,
  },
  {
    name: 'delete_issue',
    server_name: 'jira',
    approval_status: 'approved',
    enabled: true,
    description: 'Delete issue',
    annotations: { destructiveHint: true },
    is_custom_override: true,
    original_description: 'Original delete description',
  },
  {
    name: 'list_repos',
    server_name: 'github',
    approval_status: 'approved',
    enabled: true,
    description: 'List user repos',
    annotations: { readOnlyHint: true },
    is_custom_override: false,
  },
]

function mountView() {
  return mount(Tools, { global: { plugins: [createPinia()], stubs: globalStubs } })
}

describe('Tools — pattern selection and tool overrides', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    ;(api.getGlobalTools as any).mockResolvedValue({
      success: true,
      data: {
        stats: { total: 3, enabled: 3, disabled: 0, pending_approval: 0 },
        tools: TOOLS,
      },
    })
    ;(api.overrideTools as any).mockResolvedValue({ success: true })
    ;(api.resetToolOverrides as any).mockResolvedValue({ success: true })
  })

  it('selects tools by pattern matching wildcards', async () => {
    const wrapper = mountView()
    await flushPromises()

    const patternInput = wrapper.find('[data-test="pattern-select-input"]')
    expect(patternInput.exists()).toBe(true)

    // Select tools matching "*issue*"
    await patternInput.setValue('*issue*')
    await wrapper.find('[data-test="pattern-select-btn"]').trigger('click')
    await flushPromises()

    // Batch bar should show 2 tools selected
    const batchBar = wrapper.find('[data-test="tools-batch-bar"]')
    expect(batchBar.exists()).toBe(true)
    expect(batchBar.text()).toContain('2 tools selected')

    // Deselect tools matching "get_*"
    await patternInput.setValue('get_*')
    await wrapper.find('[data-test="pattern-deselect-btn"]').trigger('click')
    await flushPromises()

    // Batch bar should show 1 tool selected (delete_issue)
    expect(wrapper.find('[data-test="tools-batch-bar"]').text()).toContain('1 tool selected')
  })

  it('renders custom badge on overridden tools in the table', async () => {
    const wrapper = mountView()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('custom')
  })

  it('performs batch override and resets overrides', async () => {
    const wrapper = mountView()
    await flushPromises()

    // Select all
    await wrapper.find('[data-test="tools-select-all"]').setValue(true)
    await flushPromises()

    // Open batch override modal
    const batchOverrideBtn = wrapper.find('[data-test="batch-override"]')
    expect(batchOverrideBtn.exists()).toBe(true)
    await batchOverrideBtn.trigger('click')
    await flushPromises()

    const modal = wrapper.find('[data-test="batch-override-modal"]')
    expect(modal.exists()).toBe(true)

    // Click Read-Only preset
    await wrapper.find('[data-test="batch-preset-read"]').trigger('click')
    await flushPromises()

    // Apply overrides
    await wrapper.find('[data-test="batch-override-save"]').trigger('click')
    await flushPromises()

    expect(api.overrideTools).toHaveBeenCalled()

    // Test batch reset
    await wrapper.find('[data-test="tools-select-all"]').setValue(true)
    await flushPromises()
    await wrapper.find('[data-test="batch-reset-override"]').trigger('click')
    await flushPromises()

    expect(api.resetToolOverrides).toHaveBeenCalled()
  })

  it('allows editing overrides in detail modal and resetting to upstream', async () => {
    const wrapper = mountView()
    await flushPromises()

    // Find the row for delete_issue (which has is_custom_override: true)
    const rows = wrapper.findAll('[data-test="tool-row"]')
    expect(rows.length).toBe(3)
    const deleteRow = rows.find(r => r.text().includes('delete_issue'))
    expect(deleteRow).toBeDefined()
    await deleteRow!.trigger('click')
    await flushPromises()

    const detailModal = wrapper.find('[data-test="tool-detail-modal"]')
    expect(detailModal.exists()).toBe(true)

    // Reset button should exist for overridden tool
    const resetBtn = wrapper.find('[data-test="detail-reset-btn"]')
    expect(resetBtn.exists()).toBe(true)

    // Enter edit mode
    const editBtn = wrapper.find('[data-test="detail-edit-btn"]')
    expect(editBtn.exists()).toBe(true)
    await editBtn.trigger('click')
    await flushPromises()

    const descInput = wrapper.find('[data-test="detail-desc-input"]')
    expect(descInput.exists()).toBe(true)
    await descInput.setValue('Custom new description')

    const saveBtn = wrapper.find('[data-test="detail-save-btn"]')
    await saveBtn.trigger('click')
    await flushPromises()

    expect(api.overrideTools).toHaveBeenCalledWith(
      expect.objectContaining({
        server_name: 'jira',
        tools: ['delete_issue'],
        description: 'Custom new description',
      })
    )
  })
})
