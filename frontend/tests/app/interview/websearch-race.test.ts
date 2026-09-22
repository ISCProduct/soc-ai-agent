/**
 * WEB検索・企業一覧の「後着の古い応答が最新を上書きする」競合の回帰テスト（#1464）。
 *
 * page-content.tsx の useEffect と同じ構造を最小再現する。
 * 争点は「fetch 後の cancelled チェックだけでは足りない。r.json() も待ちで、
 * その間に別の検索が cancelled を立てる」こと。setState 直前の再チェックが要る。
 */

// effect1つ分を模した関数。page-content.tsx の修正後と同じ制御にする。
async function runSearch(
  fetchImpl: () => Promise<{ json: () => Promise<{ results: string[] }> }>,
  isCancelled: () => boolean,
  setResults: (r: string[]) => void,
) {
  const r = await fetchImpl()
  if (isCancelled()) return
  const data = await r.json()
  // 修正の要: json() の待ち後にもう一度見る
  if (isCancelled()) return
  setResults(data.results)
}

test('json() の待ち中にキャンセルされたら、古い結果で上書きしない', async () => {
  let cancelled = false
  const setResults = jest.fn()

  await runSearch(
    // fetch 完了時点ではキャンセルされていない（最初のチェックは素通り）
    async () => ({
      // json() が呼ばれた＝待ちに入った瞬間に、別検索のクリーンアップが走る状況
      json: async () => {
        cancelled = true
        return { results: ['古い結果'] }
      },
    }),
    () => cancelled,
    setResults,
  )

  expect(setResults).not.toHaveBeenCalled()
})

test('キャンセルされていなければ結果を反映する', async () => {
  const setResults = jest.fn()
  await runSearch(
    async () => ({ json: async () => ({ results: ['最新'] }) }),
    () => false,
    setResults,
  )
  expect(setResults).toHaveBeenCalledWith(['最新'])
})

test('fetch 完了時点で既にキャンセルなら json() を読まない', async () => {
  const setResults = jest.fn()
  const jsonSpy = jest.fn(async () => ({ results: ['x'] }))
  await runSearch(
    async () => ({ json: jsonSpy }),
    () => true,
    setResults,
  )
  expect(jsonSpy).not.toHaveBeenCalled()
  expect(setResults).not.toHaveBeenCalled()
})
