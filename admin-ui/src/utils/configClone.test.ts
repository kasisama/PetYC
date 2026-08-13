import { reactive } from 'vue'
import { describe, expect, it } from 'vitest'
import { cloneConfigValue } from './configClone'

describe('cloneConfigValue', () => {
  it('可复制 Vue 响应式配置行且不会保留代理引用', () => {
    const source = reactive({ name: '森林调查周', choices: ['记录线索', '继续调查', '呼叫支援'] })
    const cloned = cloneConfigValue(source)
    expect(cloned).toEqual(source)
    expect(cloned).not.toBe(source)
    cloned.choices[0] = '修改后的选项'
    expect(source.choices[0]).toBe('记录线索')
  })
})
