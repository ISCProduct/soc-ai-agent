/**
 * @jest-environment jsdom
 */
import type { CompanyCandidate } from '@/app/resume/types'
import {
  getSeverityConfig,
  isAnnotatedPdfResponse,
  mapDbCompanyResults,
  mapWebSearchResults,
  parseApiErrorMessage,
  severityConfig,
} from '@/app/resume/utils'

describe('severityConfig / getSeverityConfig', () => {
  it('既知の severity に設定を返す', () => {
    expect(severityConfig.critical).toEqual({
      color: 'error',
      label: '重大',
      borderColor: '#d32f2f',
    })
    expect(severityConfig.warning.label).toBe('注意')
    expect(severityConfig.info.color).toBe('info')
  })

  it('未知の severity にはデフォルト表示を返す', () => {
    expect(getSeverityConfig('unknown')).toEqual({
      color: 'default',
      label: 'unknown',
      borderColor: '#9e9e9e',
    })
  })

  it('既知の severity は getSeverityConfig でも同じ値', () => {
    expect(getSeverityConfig('critical')).toEqual(severityConfig.critical)
  })
})

describe('parseApiErrorMessage', () => {
  it('JSON の error フィールドを優先する', () => {
    expect(parseApiErrorMessage('{"error":"詳細エラー"}', 'デフォルト')).toBe('詳細エラー')
  })

  it('JSON の message フィールドをフォールバックする', () => {
    expect(parseApiErrorMessage('{"message":"メッセージ"}', 'デフォルト')).toBe('メッセージ')
  })

  it('JSON でない場合は生テキストを返す', () => {
    expect(parseApiErrorMessage('plain error', 'デフォルト')).toBe('plain error')
  })

  it('空文字の場合はデフォルトを返す', () => {
    expect(parseApiErrorMessage('', 'デフォルト')).toBe('デフォルト')
  })
})

describe('mapDbCompanyResults', () => {
  it('companies 配列を候補に変換する', () => {
    const list = mapDbCompanyResults({
      companies: [
        { id: 1, name: '株式会社A', description: '説明A' },
        { id: 2, name: '株式会社B' },
      ],
    })

    expect(list).toEqual<CompanyCandidate[]>([
      {
        name: '株式会社A',
        description: '説明A',
        source: 'db',
        exists: true,
        confidence: 'high',
        company_id: 1,
      },
      {
        name: '株式会社B',
        description: '',
        source: 'db',
        exists: true,
        confidence: 'high',
        company_id: 2,
      },
    ])
  })

  it('name がないエントリは除外する', () => {
    expect(mapDbCompanyResults({ companies: [{ id: 1 }, { name: '有効' }] })).toHaveLength(1)
  })

  it('配列を直接渡しても変換できる', () => {
    expect(mapDbCompanyResults([{ name: '直接' }])).toEqual([
      expect.objectContaining({ name: '直接', source: 'db' }),
    ])
  })
})

describe('mapWebSearchResults', () => {
  it('results を候補に変換する', () => {
    const list = mapWebSearchResults({
      results: [
        {
          name: 'WEB企業',
          description: 'WEB説明',
          source: 'web',
          confidence: 'medium',
          company_id: 10,
          evidence_urls: ['https://example.com'],
        },
      ],
    })

    expect(list).toEqual([
      {
        name: 'WEB企業',
        description: 'WEB説明',
        source: 'web',
        exists: true,
        confidence: 'medium',
        company_id: 10,
        evidence_urls: ['https://example.com'],
      },
    ])
  })

  it('exists: false の候補は除外する', () => {
    expect(mapWebSearchResults({
      results: [{ name: '除外', exists: false }, { name: '残る' }],
    })).toHaveLength(1)
  })

  it('source 省略時は web_search を使う', () => {
    expect(mapWebSearchResults({ results: [{ name: 'A' }] })[0].source).toBe('web_search')
  })
  it('null / 非オブジェクトは空配列', () => {
    expect(mapDbCompanyResults(null)).toEqual([])
    expect(mapDbCompanyResults('x')).toEqual([])
    expect(mapWebSearchResults(null)).toEqual([])
    expect(mapWebSearchResults(undefined)).toEqual([])
  })
})

describe('isAnnotatedPdfResponse', () => {
  it('application/pdf の Content-Type を PDF と判定する', () => {
    expect(isAnnotatedPdfResponse('application/pdf', '', 200)).toBe(true)
  })

  it('Content-Disposition に .pdf があれば PDF と判定する', () => {
    expect(isAnnotatedPdfResponse('application/octet-stream', 'attachment; filename="a.pdf"', 200)).toBe(true)
  })

  it('206 または content-length/range 付き 200 を PDF ありと判定する', () => {
    expect(isAnnotatedPdfResponse('application/octet-stream', '', 206)).toBe(true)
    expect(isAnnotatedPdfResponse('application/octet-stream', '', 200, '1', null)).toBe(true)
    expect(isAnnotatedPdfResponse('application/octet-stream', '', 200, null, 'bytes 0-0/10')).toBe(true)
    expect(isAnnotatedPdfResponse('application/octet-stream', '', 200)).toBe(false)
  })

  it('PDF でない応答は false', () => {
    expect(isAnnotatedPdfResponse('application/json', '', 404)).toBe(false)
  })
})
