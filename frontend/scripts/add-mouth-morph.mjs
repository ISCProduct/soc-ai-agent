#!/usr/bin/env node
/**
 * 骨の無いメッシュに「口が開く」モーフを後付けする。
 *
 * この顔は目も口もテクスチャに描かれているだけで、動かす立体が無い。
 * ただし口は、顎の形そのものを下げれば描かれた唇も一緒に動くので、
 * 「口が開いた」ように見せられる（テクスチャには手を触れない）。
 *
 * まばたきは同じ手が使えない。目を閉じるには肌で目を覆う必要があるが、
 * 頂点を動かしても描かれた目はUVについてくるので隠れない。
 * 閉じ目のテクスチャを用意して差し替える方式になり、絵を描く作業になる。
 *
 *   node scripts/add-mouth-morph.mjs public/avatars/male-avatar.glb
 *
 * 口の位置は解剖学的な比率から決める。テクスチャの明暗からの検出は、
 * このモデルではテクスチャ全体が暗く（明度の中央値36）判別が成立しなかった。
 */
import { readFileSync, writeFileSync } from 'node:fs'
import { basename } from 'node:path'

const GLTF_MAGIC = 0x46546c67
const COMP = { FLOAT: 5126, USHORT: 5123, UBYTE: 5121, UINT: 5125 }
const NUM = { SCALAR: 1, VEC2: 2, VEC3: 3, VEC4: 4, MAT4: 16 }
const BYTES = { 5126: 4, 5123: 2, 5121: 1, 5125: 4 }

function readGlb(path) {
  const buf = readFileSync(path)
  if (buf.readUInt32LE(0) !== GLTF_MAGIC) throw new Error('glTF バイナリではありません')
  const jsonLen = buf.readUInt32LE(12)
  const json = JSON.parse(buf.subarray(20, 20 + jsonLen).toString('utf8'))
  const p = 20 + jsonLen
  return { json, bin: buf.subarray(p + 8, p + 8 + buf.readUInt32LE(p)) }
}
function readAccessor(json, bin, index) {
  const a = json.accessors[index]
  const bv = json.bufferViews[a.bufferView]
  const n = NUM[a.type], size = BYTES[a.componentType]
  const start = (bv.byteOffset ?? 0) + (a.byteOffset ?? 0)
  const out = []
  for (let i = 0; i < a.count; i++) {
    const row = []
    for (let k = 0; k < n; k++) {
      const off = start + (i * n + k) * size
      row.push(a.componentType === COMP.FLOAT ? bin.readFloatLE(off)
        : a.componentType === COMP.USHORT ? bin.readUInt16LE(off)
        : a.componentType === COMP.UINT ? bin.readUInt32LE(off) : bin.readUInt8(off))
    }
    out.push(row)
  }
  return out
}
const align4 = (n) => (4 - (n % 4)) % 4

function addMouth(path) {
  const { json, bin } = readGlb(path)
  const mesh = json.meshes[0]
  const prim = mesh.primitives[0]
  if (prim.targets?.length > 0) {
    console.log(`${basename(path)}: すでにモーフがあります。何もしません。`)
    return
  }
  const pos = readAccessor(json, bin, prim.attributes.POSITION)

  let yMin = Infinity, yMax = -Infinity
  for (const p of pos) { if (p[1] < yMin) yMin = p[1]; if (p[1] > yMax) yMax = p[1] }
  const H = yMax - yMin

  // 顔の前面を取る。首より上・前向き・中央寄り。
  const neckY = yMin + H * 0.55
  const faceIdx = []
  for (let i = 0; i < pos.length; i++) {
    if (pos[i][1] > neckY && pos[i][2] > 0.03 && Math.abs(pos[i][0]) < 0.12) faceIdx.push(i)
  }
  if (faceIdx.length === 0) throw new Error('顔の前面を検出できませんでした')

  // 顔前面の縦範囲から口の高さを決める。
  // 下端は顎、上端は額。口は顎から上へ 18〜38% のあたりにある。
  let fMin = Infinity, fMax = -Infinity, zMax = -Infinity
  for (const i of faceIdx) {
    if (pos[i][1] < fMin) fMin = pos[i][1]
    if (pos[i][1] > fMax) fMax = pos[i][1]
    if (pos[i][2] > zMax) zMax = pos[i][2]
  }
  const faceH = fMax - fMin
  const mouthY = fMin + faceH * 0.28
  // 口の広がり。顔の幅の基準として、口の高さ付近のX幅を使う。
  let xw = 0
  for (const i of faceIdx) {
    if (Math.abs(pos[i][1] - mouthY) < faceH * 0.05) xw = Math.max(xw, Math.abs(pos[i][0]))
  }
  // 縦の影響範囲。狭いと変位の勾配が急になり、テクスチャが伸びて滲む。
  // 歪み（＝変位÷影響範囲）を3割程度に収めるため広めに取る。
  // 結果として唇だけでなく顎全体が動き、実際の口の開き方に近くなる。
  const radiusY = faceH * 0.16
  const radiusX = Math.max(xw * 0.75, 0.02)

  console.log(`${basename(path)}`)
  console.log(`  顔の前面 ${faceIdx.length.toLocaleString()} 頂点（縦 ${faceH.toFixed(3)}）`)
  console.log(`  口の中心 Y=${mouthY.toFixed(3)}（顎から ${(28).toFixed(0)}%）影響範囲 縦±${radiusY.toFixed(3)} 横±${radiusX.toFixed(3)}`)

  // 口が開く変位。口の中心からの3次元の距離でなめらかに減衰させる。
  //
  // 顔の判定（faceIdx）に入った頂点だけを動かすと、判定の境界で隣り合う
  // 頂点の一方が動かず、辺が極端に伸びてメッシュが裂ける（実測で最大1013%）。
  // 距離による減衰を全頂点に適用して、境界そのものを無くす。
  // 変位。大きくすると口は開くがテクスチャが伸びる。
  // 口の動きは「量」より「動いていること」が伝わればよいので控えめにする。
  // avatar-motion.ts は音声振幅(0〜1)でこのモーフを駆動するので、
  // 通常の発話ではこの全開値には届かない。
  const DROP = faceH * 0.05
  const PULL = faceH * 0.012
  // 口の中心は顔の前面。奥行きも減衰に含めて後頭部を動かさない。
  const mouthZ = zMax * 0.75
  const radiusZ = Math.max(zMax * 0.9, 0.03)
  const delta = new Float32Array(pos.length * 3)
  let moved = 0, maxMove = 0
  for (let i = 0; i < pos.length; i++) {
    const dx = pos[i][0] / radiusX
    const dy = (pos[i][1] - mouthY) / radiusY
    const dz = (pos[i][2] - mouthZ) / radiusZ
    const d = Math.sqrt(dx * dx + dy * dy + dz * dz)
    if (d >= 1) continue
    const t = 1 - d
    const w = t * t * (3 - 2 * t)
    // 口より下（顎側）を強く動かす。上唇より下唇が大きく動くのが自然。
    //
    // ここを三項演算子で 1.0 / 0.45 と切り替えると mouthY で段差ができ、
    // その境界をまたぐ辺が裂ける（実測で最大956%伸び、25%超が2,618本）。
    // 境界をまたぐ辺の両端でウェイトが2倍以上違うのが原因だった。
    // 口の高さを中心になめらかに移す。
    const biasT = Math.max(0, Math.min(1, (mouthY - pos[i][1]) / radiusY + 0.5))
    const bias = 0.45 + 0.55 * (biasT * biasT * (3 - 2 * biasT))
    const m = w * bias
    delta[i * 3 + 1] = -DROP * m
    delta[i * 3 + 2] = -PULL * m
    if (m > 0.01) moved++
    if (DROP * m > maxMove) maxMove = DROP * m
  }
  console.log(`  動く頂点 ${moved.toLocaleString()}  最大変位 ${maxMove.toFixed(4)}`)

  // バッファに追記
  const deltaBuf = Buffer.from(delta.buffer, delta.byteOffset, delta.byteLength)
  const pad = Buffer.alloc(align4(bin.length))
  const viewOffset = bin.length + pad.length
  json.bufferViews.push({ buffer: 0, byteOffset: viewOffset, byteLength: deltaBuf.length })
  // min/max は必須
  let mn = [0, 0, 0], mx = [0, 0, 0]
  for (let i = 0; i < pos.length; i++) {
    for (let k = 0; k < 3; k++) {
      const v = delta[i * 3 + k]
      if (v < mn[k]) mn[k] = v
      if (v > mx[k]) mx[k] = v
    }
  }
  json.accessors.push({
    bufferView: json.bufferViews.length - 1,
    componentType: COMP.FLOAT, count: pos.length, type: 'VEC3', min: mn, max: mx,
  })
  prim.targets = [{ POSITION: json.accessors.length - 1 }]
  // 'aa' は VRM / Oculus viseme と同じ命名。avatar-capabilities.ts がこの名前を照合する。
  mesh.extras = { ...(mesh.extras ?? {}), targetNames: ['aa'] }
  mesh.weights = [0]

  const newBin = Buffer.concat([bin, pad, deltaBuf])
  json.buffers[0].byteLength = newBin.length
  json.asset.generator = (json.asset.generator ?? '') + ' + add-mouth-morph'

  const jsonBuf = Buffer.from(JSON.stringify(json), 'utf8')
  const jsonChunk = Buffer.concat([jsonBuf, Buffer.alloc(align4(jsonBuf.length), 0x20)])
  const binChunk = Buffer.concat([newBin, Buffer.alloc(align4(newBin.length))])
  const header = Buffer.alloc(12)
  header.writeUInt32LE(GLTF_MAGIC, 0); header.writeUInt32LE(2, 4)
  header.writeUInt32LE(12 + 8 + jsonChunk.length + 8 + binChunk.length, 8)
  const jh = Buffer.alloc(8); jh.writeUInt32LE(jsonChunk.length, 0); jh.writeUInt32LE(0x4e4f534a, 4)
  const bh = Buffer.alloc(8); bh.writeUInt32LE(binChunk.length, 0); bh.writeUInt32LE(0x004e4942, 4)
  const out = Buffer.concat([header, jh, jsonChunk, bh, binChunk])
  writeFileSync(path, out)
  console.log(`  → ${path} (${(out.length / 1024 / 1024).toFixed(1)}MB)\n`)
}

const args = process.argv.slice(2)
const targets = args.length > 0 ? args
  : ['public/avatars/male-avatar.glb', 'public/avatars/female-avatar.glb']
for (const t of targets) addMouth(t)
console.log('まばたきは入りません（描かれた目は頂点を動かしても隠れない）。')
