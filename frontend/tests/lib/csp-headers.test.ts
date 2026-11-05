/**
 * 本番CSPの `script-src` の検査。
 *
 * Live2D の Cubism Core は `WebAssembly.instantiateStreaming` を呼ぶため、
 * `'wasm-unsafe-eval'` が無いと本番でWebAssemblyのコンパイルがCSPに拒否され、
 * 面接官の初期化が失敗する（資産を正しく配送していても Avatar2D に落ちる）。
 * 開発モードは `'unsafe-eval'` で動いてしまうので、ローカルでは気づけない。
 *
 * 逆に面接ページ以外へ広げるのは余計な緩和なので、そちらは付かないことも見る。
 */

/** next.config.ts を NODE_ENV=production で読み直して headers() を得る。 */
async function headersFor(pathPrefix: string) {
  jest.resetModules()
  const prev = process.env.NODE_ENV
  // next.config.ts は読み込み時に isDev を決めるので、代入してから import する。
  Object.defineProperty(process.env, 'NODE_ENV', { value: 'production', configurable: true })
  try {
    const config = (await import('@/next.config')).default
    const entries = await config.headers!()
    const entry = entries.find((e) => e.source.startsWith(pathPrefix))
    if (!entry) throw new Error(`${pathPrefix} のヘッダ定義が無い`)
    return entry.headers
  } finally {
    Object.defineProperty(process.env, 'NODE_ENV', { value: prev, configurable: true })
  }
}

function csp(headers: { key: string; value: string }[]): string {
  const found = headers.filter((h) => h.key === 'Content-Security-Policy')
  // 2本あると後勝ちで効く・ブラウザによっては両方が適用されるので、1本であること自体を見る。
  expect(found).toHaveLength(1)
  return found[0].value
}

describe('本番の Content-Security-Policy', () => {
  it('面接ページは WebAssembly のコンパイルを許可する', async () => {
    expect(csp(await headersFor('/interview'))).toContain("'wasm-unsafe-eval'")
  })

  it('面接ページ以外には WebAssembly の許可を広げない', async () => {
    expect(csp(await headersFor('/(.*)'))).not.toContain("'wasm-unsafe-eval'")
  })

  it('WebAssembly を許可しても JavaScript の eval は許可しない', async () => {
    expect(csp(await headersFor('/interview'))).not.toContain("'unsafe-eval'")
  })

  it('面接ページはカメラ・マイクを許可したままにする', async () => {
    const permissions = (await headersFor('/interview')).filter((h) => h.key === 'Permissions-Policy')
    expect(permissions).toHaveLength(1)
    expect(permissions[0].value).toContain('camera=(self)')
    expect(permissions[0].value).toContain('microphone=(self)')
  })
})
