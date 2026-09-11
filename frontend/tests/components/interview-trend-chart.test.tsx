/**
 * @jest-environment jsdom
 */
import fs from 'fs'
import path from 'path'

// 面接スコアは interview_report.go のプロンプトで「各スコアは0〜5の整数」と
// 定義されている。Y軸が 0〜10 のままだと満点でもグラフの中央にしか描かれず、
// 伸びているかどうかが読み取れない（#1225 で発見・修正）。
//
// recharts の描画結果からドメインを読むのは不安定なので、定義そのものを見る。
describe('InterviewTrendChart のY軸', () => {
  const src = fs.readFileSync(
    path.join(process.cwd(), 'components/InterviewTrendChart.tsx'),
    'utf-8',
  )

  it('スコアの定義域 0〜5 に合わせている', () => {
    expect(src).toContain('domain={[0, 5]}')
  })

  it('0〜10 に戻っていない', () => {
    expect(src).not.toContain('domain={[0, 10]}')
  })

  it('5段階が読めるよう目盛りを明示している', () => {
    expect(src).toContain('ticks={[0, 1, 2, 3, 4, 5]}')
  })
})

// 教員側の詳細ダイアログでも同じグラフを使う（#1225）。
// 学生だけが推移を見られて教員は数値表、という状態を解消したもの。
describe('教員ダッシュボードからの利用', () => {
  const src = fs.readFileSync(
    path.join(process.cwd(), 'app/admin/dashboard/page-content.tsx'),
    'utf-8',
  )

  it('共有コンポーネントを参照している', () => {
    expect(src).toContain("import('@/components/InterviewTrendChart')")
  })

  it('recharts を本体バンドルへ入れないため dynamic import している', () => {
    expect(src).toMatch(/dynamic\(\s*\(\)\s*=>\s*import\('@\/components\/InterviewTrendChart'\)/)
  })

  it('セッションを古い順に並べ替えてから描画する', () => {
    // detailSessions は新しい順で来る。そのまま渡すと時間が右から左へ進む
    expect(src).toContain('.reverse()')
  })
})
