import { encodePathSegments } from '@/lib/api-proxy'

describe('encodePathSegments', () => {
  it.each<[string, string[], string | null]>([
    ['正常なパス', ['jobs', '12', 'applications'], 'jobs/12/applications'],
    ['日本語はエンコードする', ['検索'], encodeURIComponent('検索')],
    ['& や ? を含む要素はエンコードする', ['a&b?c=1'], 'a%26b%3Fc%3D1'],
    ['% は文字としてエンコードする', ['100%'], '100%25'],
    ['.. は拒否', ['jobs', '..', 'admin'], null],
    ['. は拒否', ['.'], null],
    ['%2e%2e は拒否', ['%2e%2e'], null],
    ['%2E. は拒否', ['%2E.'], null],
    ['/ を含む要素は拒否', ['a/b'], null],
    ['%2F を含む要素は拒否', ['a%2Fb'], null],
    ['\\ を含む要素は拒否', ['a\\b'], null],
    ['空要素は拒否', ['jobs', ''], null],
    ['デコードできない % は文字として扱う', ['%E0%A4%A'], '%25E0%25A4%25A'],
    ['空配列は空文字', [], ''],
  ])('%s', (_name, segments, expected) => {
    expect(encodePathSegments(segments)).toBe(expected)
  })
})
