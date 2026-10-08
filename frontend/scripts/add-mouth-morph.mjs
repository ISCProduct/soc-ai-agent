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

  // 顔がどちらを向いているか。
  //
  // 鼻の突出から自動検出しようとしたが当てにならなかった（男性 -X / 女性 -Z と
  // 割れたのに、実際はどちらも +X。髪の膨らみが鼻より前に出るため）。
  // 推定に頼らず明示する。Tripo の出力は顔が +X を向いており、これは
  // add-rig.mjs が骨格へ -90度/Y を焼き込んで正面へ向けることと、
  // 実際に描画して確認した結果に基づく。
  // 別の作りのモデルを入れるときは --front=+Z のように指定する。
  const frontArg = (process.argv.find((a) => a.startsWith('--front=')) ?? '--front=+X').slice(8)
  const axis = frontArg.includes('Z') ? 'Z' : 'X'
  const sign = frontArg.startsWith('-') ? -1 : 1

  const neckY = yMin + H * 0.55
  const headIdx = []
  for (let i = 0; i < pos.length; i++) if (pos[i][1] > neckY) headIdx.push(i)
  if (headIdx.length === 0) throw new Error('頭部を検出できませんでした')
  let cx = 0, cz = 0
  for (const i of headIdx) { cx += pos[i][0]; cz += pos[i][2] }
  cx /= headIdx.length; cz /= headIdx.length

  console.log(`${basename(path)}`)
  console.log(`  顔の向き ${sign > 0 ? '+' : '-'}${axis}`)

  // 正面方向の座標。以降は「前後」をこれで扱う。
  const fwd = (p) => (axis === 'X' ? p[0] : p[2]) * sign
  // 左右方向（正面に直交する水平軸）
  const side = (p) => (axis === 'X' ? p[2] : p[0])
  const center = axis === 'X' ? cx * sign : cz * sign

  const faceIdx = []
  for (const i of headIdx) {
    if (fwd(pos[i]) - center > 0.03 && Math.abs(side(pos[i])) < 0.12) faceIdx.push(i)
  }
  if (faceIdx.length === 0) throw new Error('顔の前面を検出できませんでした')

  let fMin = Infinity, fMax = -Infinity, zMax = -Infinity
  for (const i of faceIdx) {
    if (pos[i][1] < fMin) fMin = pos[i][1]
    if (pos[i][1] > fMax) fMax = pos[i][1]
    const f = fwd(pos[i])
    if (f > zMax) zMax = f
  }
  const faceH = fMax - fMin
  const mouthY = fMin + faceH * 0.28
  let xw = 0
  for (const i of faceIdx) {
    if (Math.abs(pos[i][1] - mouthY) < faceH * 0.05) xw = Math.max(xw, Math.abs(side(pos[i])))
  }
  // 縦の影響範囲。狭いと変位の勾配が急になり、テクスチャが伸びて滲む。
  // 歪み（＝変位÷影響範囲）を3割程度に収めるため広めに取る。
  const radiusY = faceH * 0.16
  const radiusX = Math.max(xw * 0.75, 0.02)

  console.log(`  顔の前面 ${faceIdx.length.toLocaleString()} 頂点（縦 ${faceH.toFixed(3)}）`)
  console.log(`  口の中心 Y=${mouthY.toFixed(3)}（顎から28%）影響範囲 縦±${radiusY.toFixed(3)} 横±${radiusX.toFixed(3)}`)

  // 口が開く変位。口の中心からの3次元の距離でなめらかに減衰させる。
  //
  // 顔の判定（faceIdx）に入った頂点だけを動かすと、判定の境界で隣り合う
  // 頂点の一方が動かず、辺が極端に伸びてメッシュが裂ける（実測で最大1013%）。
  // 距離による減衰を全頂点に適用して、境界そのものを無くす。
  // 変位。大きくすると口は開くがテクスチャが伸びる。
  // 唇はテクスチャに描かれているだけなので、顎を大きく下げると口が開くより先に
  // 唇の絵が一緒に伸びて滲む。音声振幅は通常の発話で 1.0 まで振れる
  // （useInterviewSession が rms*6 でクリップする）ので、全開の値そのものを
  // 「ずっと出ていても見られる」大きさに抑える。
  // 開き量は顔の造りで見え方が変わるのでモデルごとに指定する。
  const dropArg = process.argv.find((a) => a.startsWith('--drop='))
  const DROP = faceH * (dropArg ? parseFloat(dropArg.slice(7)) : 0.055)
  const PULL = DROP * 0.22
  const mouthZ = zMax * 0.75
  const radiusZ = Math.max(zMax * 0.9, 0.03)
  const delta = new Float32Array(pos.length * 3)
  let moved = 0, maxMove = 0
  for (let i = 0; i < pos.length; i++) {
    const dx = side(pos[i]) / radiusX
    const dy = (pos[i][1] - mouthY) / radiusY
    const dz = (fwd(pos[i]) - mouthZ) / radiusZ
    const d = Math.sqrt(dx * dx + dy * dy + dz * dz)
    if (d >= 1) continue
    const t = 1 - d
    const w = t * t * (3 - 2 * t)
    // 口より下（顎側）を強く動かす。段差を作ると辺が裂けるのでなめらかに。
    const biasT = Math.max(0, Math.min(1, (mouthY - pos[i][1]) / radiusY + 0.5))
    const bias = 0.45 + 0.55 * (biasT * biasT * (3 - 2 * biasT))
    const m = w * bias
    delta[i * 3 + 1] = -DROP * m
    // 奥へ引く向きは正面方向の逆
    const pull = -PULL * m * sign
    if (axis === 'X') delta[i * 3 + 0] = pull
    else delta[i * 3 + 2] = pull
    if (m > 0.01) moved++
    if (DROP * m > maxMove) maxMove = DROP * m
  }
  console.log(`  動く頂点 ${moved.toLocaleString()}  最大変位 ${maxMove.toFixed(4)}`)

  // min/max は必須
  let mn = [0, 0, 0], mx = [0, 0, 0]
  for (let i = 0; i < pos.length; i++) {
    for (let k = 0; k < 3; k++) {
      const v = delta[i * 3 + k]
      if (v < mn[k]) mn[k] = v
      if (v > mx[k]) mx[k] = v
    }
  }

  // sparse accessor で書く。動くのは全頂点の2%ほどなので、
  // 全頂点ぶんを密に持つと 2.7MB 増える（sparse なら 70KB 程度）。
  // 動かない頂点は bufferView を省いた既定値（ゼロ）になる。
  const moving = []
  for (let i = 0; i < pos.length; i++) {
    if (delta[i * 3] !== 0 || delta[i * 3 + 1] !== 0 || delta[i * 3 + 2] !== 0) moving.push(i)
  }
  if (moving.length === 0) throw new Error('動く頂点が1つもありません')
  const idxBuf = Buffer.alloc(moving.length * 4)
  const valBuf = Buffer.alloc(moving.length * 3 * 4)
  moving.forEach((vi, k) => {
    idxBuf.writeUInt32LE(vi, k * 4)
    for (let c = 0; c < 3; c++) valBuf.writeFloatLE(delta[vi * 3 + c], (k * 3 + c) * 4)
  })

  // バッファに追記
  const chunks = []
  let offset = bin.length
  const appendView = (buf) => {
    const pad = align4(offset)
    if (pad > 0) { chunks.push(Buffer.alloc(pad)); offset += pad }
    json.bufferViews.push({ buffer: 0, byteOffset: offset, byteLength: buf.length })
    chunks.push(buf); offset += buf.length
    return json.bufferViews.length - 1
  }
  const idxView = appendView(idxBuf)
  const valView = appendView(valBuf)

  json.accessors.push({
    componentType: COMP.FLOAT, count: pos.length, type: 'VEC3', min: mn, max: mx,
    sparse: {
      count: moving.length,
      indices: { bufferView: idxView, byteOffset: 0, componentType: COMP.UINT },
      values: { bufferView: valView, byteOffset: 0 },
    },
  })
  prim.targets = [{ POSITION: json.accessors.length - 1 }]
  // 'aa' は VRM / Oculus viseme と同じ命名。avatar-capabilities.ts がこの名前を照合する。
  mesh.extras = { ...(mesh.extras ?? {}), targetNames: ['aa'] }
  mesh.weights = [0]

  const newBin = Buffer.concat([bin, ...chunks])
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

// --drop= などのオプションをファイル名として開こうとしないよう外す
const args = process.argv.slice(2).filter((a) => !a.startsWith('--'))
const targets = args.length > 0 ? args
  : ['public/avatars/male-avatar.glb', 'public/avatars/female-avatar.glb']
for (const t of targets) addMouth(t)
console.log('まばたきは入りません（描かれた目は頂点を動かしても隠れない）。')
