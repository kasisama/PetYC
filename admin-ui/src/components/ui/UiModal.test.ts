import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import UiModal from './UiModal.vue'

describe('UiModal', () => {
  it('按 Escape 触发关闭', async () => {
    const wrapper = mount(UiModal, {
      props: { open: true, title: '确认操作' },
      attachTo: document.body,
    })

    document.querySelector('.ui-modal-mask')?.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }),
    )
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })

  it('忙碌状态下不允许关闭', async () => {
    const wrapper = mount(UiModal, {
      props: { open: true, title: '确认操作', busy: true },
      attachTo: document.body,
    })

    document.querySelector('.ui-modal-mask')?.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }),
    )
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('close')).toBeUndefined()
    wrapper.unmount()
  })
})
