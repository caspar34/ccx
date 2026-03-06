import { describe, it, expect } from 'vitest'
import { buildExpectedRequestUrls } from './expectedRequestUrls'

describe('buildExpectedRequestUrls', () => {
  it('应为 responses 渠道上的 gemini 上游生成正确预览 URL', () => {
    const result = buildExpectedRequestUrls('responses', 'gemini', 'https://generativelanguage.googleapis.com')

    expect(result).toHaveLength(1)
    expect(result[0].expectedUrl).toBe(
      'https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent'
    )
  })

  it('应在 baseUrl 已含版本前缀时避免重复追加版本', () => {
    const result = buildExpectedRequestUrls('responses', 'gemini', 'https://generativelanguage.googleapis.com/v1beta')

    expect(result).toHaveLength(1)
    expect(result[0].expectedUrl).toBe(
      'https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent'
    )
  })
})
