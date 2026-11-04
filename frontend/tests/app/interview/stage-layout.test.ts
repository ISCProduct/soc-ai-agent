/** @jest-environment node */

import { readFileSync } from 'fs'
import { join } from 'path'

/**
 * 面接官ステージのレイアウトを、壊れ方の原因になった2点について検査する。
 *
 * 1. 上段ラベルと PiP の幅が別々の値に分かれないこと。
 *    `.stageTop` の右インセットが 240px 固定で `.pipCard` の幅が `clamp(120px, 28%, 224px)`
 *    だったため、PiP が縮む幅でも 240px 空け、nowrap + ellipsis の面接官ラベル（会社名）が
 *    無駄に切れていた。同じ custom property から配ることを見る。
 * 2. 狭い画面で自分の映像と字幕がステージの中に残らないこと。
 *    PiP は幅360pxの電話では約120pxになり、面接練習の目的である姿勢の確認ができない。
 *    字幕は最大3行あり、ステージ下部の大半を覆って面接官の口を隠す。どちらも狭い画面では
 *    ステージの外（下）に出す。
 *
 * 計算結果ではなく「1箇所から配っているか」「重ねるのは広い画面だけか」を見る。
 */
const css = readFileSync(
  join(__dirname, '../../../app/interview/components/interview.module.css'),
  'utf8',
)

/** セレクタ1つぶんの宣言ブロックを、指定した範囲の中から取り出す。 */
function block(selector: string, scope = css): string {
  const m = scope.match(new RegExp(`\\${selector}\\s*\\{([^}]*)\\}`))
  if (!m) throw new Error(`${selector} が見つからない`)
  return m[1]
}

/**
 * `@media (min-width: Npx)` のうち、指定セレクタを含むブロックの中身を返す。
 * この幅の @media はファイル内に複数あるので、セレクタで選ぶ。
 */
function mediaBlock(minWidthPx: number, selector: string): string {
  const head = `@media (min-width: ${minWidthPx}px) {`
  for (let at = css.indexOf(head); at >= 0; at = css.indexOf(head, at + 1)) {
    const open = css.indexOf('{', at)
    let depth = 0
    for (let i = open; i < css.length; i++) {
      if (css[i] === '{') depth++
      else if (css[i] === '}' && --depth === 0) {
        const body = css.slice(open + 1, i)
        if (body.includes(`${selector} {`) || body.includes(`${selector}{`)) return body
        break
      }
    }
  }
  throw new Error(`@media (min-width: ${minWidthPx}px) の中に ${selector} が見つからない`)
}

/** 自分のカメラと字幕をステージに重ねる下限幅。CSS側を変えたらここも変える。 */
const OVERLAY_MIN_WIDTH = 600

describe('面接官ステージのレイアウト', () => {
  it('狭い画面では自分のカメラをステージの外にフル幅で置く', () => {
    const pip = block('.pipCard')
    expect(pip).toMatch(/position:\s*relative/)
    expect(pip).toMatch(/width:\s*100%/)
    expect(pip).toMatch(/aspect-ratio:\s*16\s*\/\s*9/)
  })

  it('狭い画面では字幕をステージの外に置く（面接官の口を隠さない）', () => {
    const caption = block('.captionBox')
    expect(caption).toMatch(/position:\s*relative/)
    expect(caption).not.toMatch(/bottom:/)
  })

  it('広い画面だけ PiP として重ね、幅は --pip-width から取る', () => {
    const pip = block('.pipCard', mediaBlock(OVERLAY_MIN_WIDTH, '.pipCard'))
    expect(pip).toMatch(/position:\s*absolute/)
    expect(pip).toMatch(/width:\s*var\(--pip-width\)/)
    const caption = block('.captionBox', mediaBlock(OVERLAY_MIN_WIDTH, '.captionBox'))
    expect(caption).toMatch(/position:\s*absolute/)
  })

  it('stageTop の右インセットも同じ --pip-width から計算する', () => {
    const stageTop = block('.stageTop', mediaBlock(OVERLAY_MIN_WIDTH, '.stageTop'))
    expect(stageTop).toMatch(/right:\s*calc\([^)]*var\(--pip-width\)/)
    // 固定値に戻すと、PiP が縮む幅でラベルが無駄に切れる
    expect(stageTop).not.toMatch(/right:\s*\d+px/)
  })

  it('--pip-width は stageCard に1箇所だけ定義する', () => {
    expect(block('.stageCard')).toMatch(/--pip-width:\s*clamp\(/)
    const defs = css.match(/--pip-width:/g) ?? []
    expect(defs).toHaveLength(1)
  })
})
