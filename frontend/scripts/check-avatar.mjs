#!/usr/bin/env node
/**
 * アバターファイル(.vrm / .glb)が面接官として動けるかを調べる。
 *
 * ブラウザで開く前に手元で判定できるようにする。実際に本番へ入っていた
 * Tripo 製の GLB は骨格も表情も持たない静止メッシュで、置いてから
 * 「うなずかない」と気付くまで分からなかった。
 *
 *   node scripts/check-avatar.mjs public/avatars/male-avatar.vrm
 *   node scripts/check-avatar.mjs            # public/avatars/ を全部見る
 *
 * glTF のバイナリ(glb/vrm)は先頭に JSON チャンクがあるので、
 * 3Dライブラリを読み込まずにヘッダだけで判定できる。
 */
import { readFileSync, existsSync, readdirSync } from 'node:fs'
import { join, basename } from 'node:path'

const AVATAR_DIR = 'public/avatars'

/** glb/vrm の JSON チャンクを取り出す。 */
function readGltfJson(path) {
  const buf = readFileSync(path)
  if (buf.length < 20) throw new Error('ファイルが小さすぎます')
  const magic = buf.readUInt32LE(0)
  // 'glTF' = 0x46546C67
  if (magic !== 0x46546c67) {
    // .gltf（JSON そのまま）も受ける
    try {
      return JSON.parse(buf.toString('utf8'))
    } catch {
      throw new Error('glTF バイナリではありません（glb / vrm を指定してください）')
    }
  }
  const chunkLen = buf.readUInt32LE(12)
  return JSON.parse(buf.subarray(20, 20 + chunkLen).toString('utf8'))
}

const HEAD_WORDS = ['head', 'neck', '頭', '首']
const BLINK_WORDS = ['blink', 'eyeblink', 'eyesclosed', 'まばたき']
const MOUTH_WORDS = ['aa', 'mouth_open', 'mouthopen', 'jaw_open', 'jawopen', 'あ']

const norm = (s) => String(s).toLowerCase().replace(/[\s_.\-:]/g, '')

function inspect(j) {
  const nodes = (j.nodes ?? []).map((n) => n.name ?? '')
  const skins = (j.skins ?? []).length
  const animations = (j.animations ?? []).length

  // モーフ名は mesh.extras.targetNames か primitive.extras.targetNames に入る
  const morphNames = []
  for (const m of j.meshes ?? []) {
    for (const nm of m.extras?.targetNames ?? []) morphNames.push(nm)
    for (const p of m.primitives ?? []) {
      for (const nm of p.extras?.targetNames ?? []) morphNames.push(nm)
    }
  }
  let morphCount = 0
  for (const m of j.meshes ?? []) {
    for (const p of m.primitives ?? []) morphCount += (p.targets ?? []).length
  }

  // VRM の拡張。1.0 は VRMC_vrm、0.x は VRM
  const ext = j.extensions ?? {}
  const vrm1 = ext.VRMC_vrm
  const vrm0 = ext.VRM
  const vrmVersion = vrm1 ? '1.0' : vrm0 ? '0.x' : null

  // VRM の人型ボーン割り当て
  let humanoidHead = null
  if (vrm1?.humanoid?.humanBones) {
    humanoidHead = vrm1.humanoid.humanBones.head ? 'head' : (vrm1.humanoid.humanBones.neck ? 'neck' : null)
  } else if (vrm0?.humanoid?.humanBones) {
    const names = vrm0.humanoid.humanBones.map((b) => b.bone)
    humanoidHead = names.includes('head') ? 'head' : (names.includes('neck') ? 'neck' : null)
  }

  // VRM の表情
  const vrmExpr = []
  if (vrm1?.expressions?.preset) vrmExpr.push(...Object.keys(vrm1.expressions.preset))
  if (vrm0?.blendShapeMaster?.blendShapeGroups) {
    vrmExpr.push(...vrm0.blendShapeMaster.blendShapeGroups.map((g) => g.presetName || g.name))
  }

  const hit = (list, words) => list.filter((n) => words.some((w) => norm(n).includes(norm(w))))
  // 実行時(avatar-capabilities.ts findBone)と同じ優先順にする。
  // 単語の並び順が優先度で、Head と Neck の両方があるモデルでは Head を使う。
  // ここだけ走査順で先に出たものを表示すると、実行時と食い違って誤解を生む。
  const hitOrdered = (list, words) => {
    for (const w of words) {
      const exact = list.find((n) => norm(n) === norm(w))
      if (exact) return exact
    }
    for (const w of words) {
      const partial = list.find((n) => norm(n).includes(norm(w)))
      if (partial) return partial
    }
    return null
  }

  return {
    skins, animations, morphCount, vrmVersion,
    nodeCount: nodes.length,
    headBone: humanoidHead ?? hitOrdered(nodes, HEAD_WORDS),
    blink: [...hit(vrmExpr, BLINK_WORDS), ...hit(morphNames, BLINK_WORDS)],
    mouth: [...hit(vrmExpr, MOUTH_WORDS), ...hit(morphNames, MOUTH_WORDS)],
    jaw: hit(nodes, ['jaw', '顎']),
    generator: j.asset?.generator ?? '(不明)',
  }
}

function report(path) {
  let r
  try {
    r = inspect(readGltfJson(path))
  } catch (e) {
    console.log(`\n✗ ${basename(path)}: 読めません — ${e.message}`)
    return false
  }

  const problems = []
  if (r.skins === 0) problems.push('骨格(skin)が無い。静止メッシュなので全身が動かせない')
  if (!r.headBone) problems.push('頭・首のボーンが無いのでうなずけない')
  if (r.mouth.length === 0 && r.jaw.length === 0) problems.push('口の表情も顎ボーンも無いので口が動かない')
  if (r.blink.length === 0) problems.push('まばたきの表情が無い')

  console.log(`\n${problems.length === 0 ? '✓' : '✗'} ${basename(path)}`)
  console.log(`    作成ツール   ${r.generator}`)
  console.log(`    形式         ${r.vrmVersion ? `VRM ${r.vrmVersion}` : 'glTF（VRMではない）'}`)
  console.log(`    骨格(skin)   ${r.skins}`)
  console.log(`    ノード数     ${r.nodeCount}`)
  console.log(`    モーフ数     ${r.morphCount}`)
  console.log(`    頭ボーン     ${r.headBone ?? '―'}`)
  console.log(`    まばたき     ${r.blink.length > 0 ? r.blink.slice(0, 3).join(', ') : '―'}`)
  console.log(`    口           ${r.mouth.length > 0 ? r.mouth.slice(0, 3).join(', ') : (r.jaw[0] ? `顎ボーン ${r.jaw[0]}` : '―')}`)

  if (problems.length > 0) {
    console.log('\n    足りないもの:')
    for (const p of problems) console.log(`      - ${p}`)
    console.log('\n    VRoid Studio で書き出すときに「表情（ブレンドシェイプ）を削減しない」設定にしてください。')
    console.log('    詳細は public/avatars/README.md を参照。')
  }
  return problems.length === 0
}

const args = process.argv.slice(2)
const targets = args.length > 0
  ? args
  : (existsSync(AVATAR_DIR)
      ? readdirSync(AVATAR_DIR).filter((f) => /\.(vrm|glb|gltf)$/i.test(f)).map((f) => join(AVATAR_DIR, f))
      : [])

if (targets.length === 0) {
  console.log(`${AVATAR_DIR} にアバターファイル(.vrm / .glb)がありません。`)
  console.log('public/avatars/README.md の手順で用意してください。')
  process.exit(1)
}

let allOk = true
for (const t of targets) allOk = report(t) && allOk
console.log(allOk ? '\nすべて要件を満たしています。' : '\n要件を満たしていないファイルがあります。')
process.exit(allOk ? 0 : 1)
