/** @jest-environment node */

import { execFileSync } from 'child_process'
import { mkdtempSync, writeFileSync } from 'fs'
import { tmpdir } from 'os'
import { join, resolve } from 'path'

/**
 * check:live2d の合否判定。
 *
 * `.cdi3.json` が無いと、スクリプトが読めるのは model3.json の Groups 申告だけで、
 * パラメータの実体一覧が取れない。以前はそれでも「うなずきできない」と断定して
 * 落としていたが、実行時は MOC から ParamAngleY / ParamAngleX を列挙するため、
 * 同じモデルでも結果が食い違い、実際には動くモデルを落としていた。
 * cdi3 が無いときは未確認（終了コード3）として扱い、落とさない。
 */
const SCRIPT = resolve(__dirname, '../../../scripts/check-live2d.mjs')

const MODEL = {
  FileReferences: {
    Groups: [
      { Name: 'LipSync', Ids: ['ParamMouthOpenY'] },
      { Name: 'EyeBlink', Ids: ['ParamEyeLOpen', 'ParamEyeROpen'] },
    ],
  },
}

/** モデル一式を置いたディレクトリを作る。cdi は渡したときだけ書く。 */
function modelDir(cdiParameterIds?: string[]): string {
  const dir = mkdtempSync(join(tmpdir(), 'live2d-check-'))
  writeFileSync(join(dir, 't.model3.json'), JSON.stringify(MODEL))
  if (cdiParameterIds) {
    writeFileSync(
      join(dir, 't.cdi3.json'),
      JSON.stringify({ Parameters: cdiParameterIds.map((Id) => ({ Id })) }),
    )
  }
  return dir
}

/** スクリプトを動かして終了コードと出力を返す。 */
function run(dir: string): { code: number; out: string } {
  try {
    const out = execFileSync(
      process.execPath,
      ['--no-warnings=MODULE_TYPELESS_PACKAGE_JSON', SCRIPT, dir],
      { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] },
    )
    return { code: 0, out }
  } catch (e) {
    const err = e as { status: number; stdout?: string; stderr?: string }
    return { code: err.status, out: `${err.stdout ?? ''}${err.stderr ?? ''}` }
  }
}

describe('check:live2d', () => {
  it('うなずけるモデルは合格する', () => {
    const { code, out } = run(modelDir(['ParamMouthOpenY', 'ParamEyeLOpen', 'ParamEyeROpen', 'ParamAngleY']))
    expect(code).toBe(0)
    expect(out).toContain('面接官として使えます')
  })

  it('cdi3 があってうなずけないモデルは落とす', () => {
    const { code, out } = run(modelDir(['ParamMouthOpenY', 'ParamEyeLOpen', 'ParamEyeROpen']))
    expect(code).toBe(1)
    expect(out).toContain('面接官としては使えません')
  })

  it('cdi3 が無いモデルは「できない」と断定せず未確認にする', () => {
    const { code, out } = run(modelDir())
    expect(code).toBe(3)
    expect(out).toContain('判定できません')
    expect(out).toContain('未確認')
    // 実行時は MOC から列挙されるので、ここで落としてはいけない
    expect(code).not.toBe(1)
    expect(out).not.toContain('面接官としては使えません')
  })

  it('model3.json が無ければ落とす', () => {
    const dir = mkdtempSync(join(tmpdir(), 'live2d-check-empty-'))
    const { code, out } = run(dir)
    expect(code).toBe(1)
    expect(out).toContain('.model3.json がありません')
  })
})
