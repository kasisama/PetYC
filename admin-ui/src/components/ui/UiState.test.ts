import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import UiState from './UiState.vue'

describe('UiState', () => {
  it('错误状态提供重试入口', async () => {
    const wrapper = mount(UiState, {
      props: {
        tone: 'error',
        title: '加载失败',
        description: '无法读取数据',
        actionLabel: '重试',
      },
    })

    await wrapper.get('button').trigger('click')

    expect(wrapper.text()).toContain('无法读取数据')
    expect(wrapper.emitted('action')).toHaveLength(1)
  })
})
