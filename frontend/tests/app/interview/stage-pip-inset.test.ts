/** @jest-environment node */

import { readFileSync } from 'fs'
import { join } from 'path'

/**
 * 面接官ステージの上段ラベルと PiP の幅が別々の値に分かれないことを検査する。
 *
 * `.stageTop` の右インセットが 240px 固定で、`.pipCard` の幅が
 * `clamp(120px, 28%, 224px)` だった。PiP が縮む狭い画面でも 240px 空けるため、
 * nowrap + ellipsis の面接官ラベル（会社名）が無駄に切れていた。
 * 2つの値が独立していると再発するので、同じ custom property から配ることを見る。
 * 計算結果ではなく「1箇所から配っているか」を見るのは、ここが壊れ方の原因だから。
 */
const css = readFileSync(
  join(__dirname, '../../../app/interview/components/interview.module.css'),
  'utf8',
)

/** セレクタ1つぶんの宣言ブロックを取り出す。 */
function block(selector: string): string {
  const m = css.match(new RegExp(`\\${selector}\\s*\\{([^}]*)\\}`))
  if (!m) throw new Error(`${selector} が見つからない`)
  return m[1]
}

describe('面接官ステージの PiP 幅', () => {
  it('PiP の幅は --pip-width から取る', () => {
    expect(block('.pipCard')).toMatch(/width:\s*var\(--pip-width\)/)
  })

  it('stageTop の右インセットも --pip-width から計算する', () => {
    const stageTop = block('.stageTop')
    expect(stageTop).toMatch(/right:\s*calc\([^)]*var\(--pip-width\)/)
    // 固定値に戻すと狭い画面でラベルが切れる
    expect(stageTop).not.toMatch(/right:\s*\d+px/)
  })

  it('--pip-width は stageCard に1箇所だけ定義する', () => {
    expect(block('.stageCard')).toMatch(/--pip-width:\s*clamp\(/)
    const defs = css.match(/--pip-width:/g) ?? []
    expect(defs).toHaveLength(1)
  })
})
