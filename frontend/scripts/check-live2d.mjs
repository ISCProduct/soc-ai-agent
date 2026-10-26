/**
 * 置いた Live2D モデルが、うちの音声パイプラインで動くかを調べる。
 *
 *   npm run check:live2d -- public/live2d/Hiyori
 *
 * 3D版で male-avatar.glb（骨もモーフも無い静止メッシュ）を本番に入れてしまい、
 * うなずきも口パクも動いていないことに長く気付けなかった。同じ事故を、
 * モデルを差し替えた直後に気付けるようにするためのスクリプト。
 */
import { readFileSync, readdirSync } from 'node:fs'
import { basename, join } from 'node:path'
import { inspectLive2DModel, isUsableAsInterviewer, DEFICIENCY_REASON }
  from '../lib/interview/live2d-params.ts'

const dir = process.argv[2]
if (!dir) {
  console.error('使い方: npm run check:live2d -- <モデルのディレクトリ>')
  process.exit(2)
}

const files = readdirSync(dir)
const find = (suffix) => files.find((f) => f.endsWith(suffix))

const modelFile = find('.model3.json')
if (!modelFile) {
  console.error(`✗ ${dir} に .model3.json がありません。モデル一式が揃っていません。`)
  process.exit(1)
}
const model = JSON.parse(readFileSync(join(dir, modelFile), 'utf8'))
const groups = model.FileReferences?.Groups ?? model.Groups ?? []
const groupIds = (name) => groups.find((g) => g.Name === name)?.Ids ?? []

// パラメータの実体一覧は .cdi3.json（Cubism Editor が書き出す表示情報）から取る。
const cdiFile = find('.cdi3.json')
let parameterIds = []
if (cdiFile) {
  const cdi = JSON.parse(readFileSync(join(dir, cdiFile), 'utf8'))
  parameterIds = (cdi.Parameters ?? []).map((p) => p.Id)
} else {
  // cdi3 が無いモデルもある。その場合は申告されたIDだけを実体とみなす
  // （過小評価になるので、判定が緩くならない方向に倒れる）。
  parameterIds = [...new Set([...groupIds('LipSync'), ...groupIds('EyeBlink')])]
}

const caps = inspectLive2DModel({
  parameterIds,
  lipSyncIds: groupIds('LipSync'),
  eyeBlinkIds: groupIds('EyeBlink'),
})

console.log(`モデル: ${basename(dir)} (${modelFile})`)
console.log(`  パラメータ数   : ${parameterIds.length}${cdiFile ? '' : ' (cdi3.json が無いため申告分のみ)'}`)
console.log(`  LipSync 申告   : ${JSON.stringify(groupIds('LipSync'))}`)
console.log(`  口の駆動方式   : ${caps.mouth}`)
console.log(`  口に流すID     : ${JSON.stringify(caps.mouthParamIds)}`)
console.log(`  まばたき       : ${caps.blinkParamIds.length > 0 ? JSON.stringify(caps.blinkParamIds) : 'できない'}`)
console.log(`  うなずき       : ${caps.canNod ? 'できる' : 'できない'}`)
console.log(`  physics3.json  : ${find('.physics3.json') ? 'あり（揺れものが動く）' : 'なし'}`)

if (caps.deficiencies.length > 0) {
  console.log('\n注意:')
  for (const d of caps.deficiencies) console.log(`  - [${d}] ${DEFICIENCY_REASON[d]}`)
}

if (!isUsableAsInterviewer(caps)) {
  console.error('\n✗ 面接官としては使えません。口が動かないか、うなずけません。')
  process.exit(1)
}
console.log(`\n✓ 面接官として使えます${caps.deficiencies.length > 0 ? '（上の注意つき）' : ''}。`)
