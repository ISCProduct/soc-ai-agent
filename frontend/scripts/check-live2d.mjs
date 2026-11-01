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
//
// cdi3 が無いモデルもある。その場合は申告されたIDしか分からないので、
// 「パラメータが存在するか」を根拠にする判定（うなずき・まばたき・LipSync が空のときの口）は
// 「無い」ではなく「未確認」になる。実行時は MOC から全パラメータを列挙するため、
// ここで「できない」と断定すると、実際には動くモデルを落として結果が食い違う。
const cdiFile = find('.cdi3.json')
const paramsKnown = Boolean(cdiFile)
let parameterIds = []
if (paramsKnown) {
  const cdi = JSON.parse(readFileSync(join(dir, cdiFile), 'utf8'))
  parameterIds = (cdi.Parameters ?? []).map((p) => p.Id)
} else {
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
// cdi3 が無いときは「できない」ではなく「未確認」と出す。
const unknown = paramsKnown ? 'できない' : '未確認（cdi3.json が無い）'
console.log(`  まばたき       : ${caps.blinkParamIds.length > 0 ? JSON.stringify(caps.blinkParamIds) : unknown}`)
console.log(`  うなずき       : ${caps.canNod ? 'できる' : unknown}`)
console.log(`  physics3.json  : ${find('.physics3.json') ? 'あり（揺れものが動く）' : 'なし'}`)

if (caps.deficiencies.length > 0) {
  console.log('\n注意:')
  for (const d of caps.deficiencies) console.log(`  - [${d}] ${DEFICIENCY_REASON[d]}`)
}

if (!isUsableAsInterviewer(caps)) {
  if (paramsKnown) {
    console.error('\n✗ 面接官としては使えません。口が動かないか、うなずけません。')
    process.exit(1)
  }
  // 根拠となるパラメータ一覧が無いので落とさない。実機で確かめる案件として返す。
  console.error(
    '\n△ 判定できません。cdi3.json が無く、パラメータの実体一覧が取れません。' +
    '\n   Cubism Editor から cdi3.json を書き出して再実行するか、' +
    '\n   面接画面（16:9ステージ）で口パクと相槌のうなずきを目で確認してください。',
  )
  process.exit(3)
}
console.log(`\n✓ 面接官として使えます${caps.deficiencies.length > 0 ? '（上の注意つき）' : ''}。`)
