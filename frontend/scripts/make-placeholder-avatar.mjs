#!/usr/bin/env node
/**
 * 動作確認用の仮アバターを生成する。
 *
 * 面接官が「うなずく・まばたきする・口が動く」ことを実機で確認するには、
 * 骨格と表情を持つモデルが必要になる。見た目の良いモデルは造形作業なので
 * 用意に時間がかかるが、動きの確認はそれを待たずにできる。
 *
 * ここで作るのは箱を組んだだけの粗いモデルで、本番の面接画面に出すものではない。
 * 目的は2つ。
 *   - 動きのパイプライン（頭ボーン・まばたき・口）が実際に動くことの確認
 *   - E2E / 目視確認のための固定データ
 *
 * VRM の拡張は入れない。ボーン名を Head / Neck、モーフ名を blink / aa に
 * しておけば avatar-capabilities.ts の汎用経路（名前の照合）で拾われる。
 *
 *   node scripts/make-placeholder-avatar.mjs
 */
import { writeFileSync, mkdirSync } from 'node:fs'
import { dirname } from 'node:path'

// ─── glTF を手で組む ─────────────────────────────────────────────────────────
// 3Dライブラリを使わないのは、Node で GLTFExporter を動かすのに
// ブラウザAPIの代用が必要になり、生成物より足場のほうが複雑になるため。

/** 箱の頂点を作る。cx,cy,cz 中心、w,h,d の大きさ。 */
function box(cx, cy, cz, w, h, d) {
  const x = w / 2, y = h / 2, z = d / 2
  const p = [
    [-x, -y, -z], [x, -y, -z], [x, y, -z], [-x, y, -z],
    [-x, -y, z], [x, -y, z], [x, y, z], [-x, y, z],
  ].map(([a, b, c]) => [a + cx, b + cy, c + cz])
  const idx = [
    0, 1, 2, 0, 2, 3, // back
    4, 6, 5, 4, 7, 6, // front
    0, 4, 5, 0, 5, 1, // bottom
    3, 2, 6, 3, 6, 7, // top
    0, 3, 7, 0, 7, 4, // left
    1, 5, 6, 1, 6, 2, // right
  ]
  return { p, idx }
}

const positions = []
const indices = []
const joints = []   // 各頂点が従うジョイント番号
const weights = []  // そのウェイト（1.0固定の単一バインド）

/** 箱を1つ足して、全頂点を指定ジョイントにバインドする。 */
function addBox(b, joint) {
  const base = positions.length
  for (const v of b.p) { positions.push(v); joints.push(joint); weights.push(1) }
  for (const i of b.idx) indices.push(base + i)
  return { from: base, to: positions.length }
}

// ジョイント: 0=Hips 1=Spine 2=Neck 3=Head
// 身長は 1.7 程度にする。ThreeAvatar 側が自動で 1.8 に正規化する。
addBox(box(0, 0.45, 0, 0.42, 0.9, 0.22), 1)   // 胴
addBox(box(0, 0.98, 0, 0.14, 0.16, 0.14), 2)  // 首
const head = addBox(box(0, 1.28, 0, 0.36, 0.44, 0.32), 3) // 頭

// 目と口。まばたき・口の動きが見えるように、頭の前面へ小さな箱を足す。
const eyeL = addBox(box(-0.09, 1.34, 0.17, 0.09, 0.05, 0.03), 3)
const eyeR = addBox(box(0.09, 1.34, 0.17, 0.09, 0.05, 0.03), 3)
const mouth = addBox(box(0, 1.17, 0.17, 0.14, 0.04, 0.03), 3)

const vertexCount = positions.length

/** モーフ: 指定範囲の頂点だけを動かす差分を作る。 */
function morph(range, dx, dy, dz) {
  const out = new Array(vertexCount).fill(null).map(() => [0, 0, 0])
  for (let i = range.from; i < range.to; i++) out[i] = [dx, dy, dz]
  return out
}

// まばたき: 目を縦に潰す（下へ寄せる）
const blinkDelta = morph(eyeL, 0, -0.025, 0).map((v, i) =>
  i >= eyeR.from && i < eyeR.to ? [0, -0.025, 0] : v)
// 口: 下へ開く
const aaDelta = morph(mouth, 0, -0.05, 0)

// ─── バイナリに詰める ───────────────────────────────────────────────────────
const chunks = []
let offset = 0
function push(typedArray) {
  const buf = Buffer.from(typedArray.buffer, typedArray.byteOffset, typedArray.byteLength)
  // 4バイト境界に揃える
  const pad = (4 - (buf.length % 4)) % 4
  const view = { byteOffset: offset, byteLength: buf.length }
  chunks.push(buf)
  if (pad > 0) chunks.push(Buffer.alloc(pad))
  offset += buf.length + pad
  return view
}

const flat = (arr) => Float32Array.from(arr.flat())
const minMax = (arr) => {
  const min = [Infinity, Infinity, Infinity], max = [-Infinity, -Infinity, -Infinity]
  for (const v of arr) for (let i = 0; i < 3; i++) {
    if (v[i] < min[i]) min[i] = v[i]
    if (v[i] > max[i]) max[i] = v[i]
  }
  return { min, max }
}

const vPos = push(flat(positions))
const vIdx = push(Uint16Array.from(indices))
const vJoint = push(Uint16Array.from(joints.flatMap((j) => [j, 0, 0, 0])))
const vWeight = push(Float32Array.from(weights.flatMap((w) => [w, 0, 0, 0])))
const vBlink = push(flat(blinkDelta))
const vAa = push(flat(aaDelta))

// 逆バインド行列。各ジョイントのワールド位置の逆を単位行列へ入れる。
// ジョイントは全て原点基準のローカル移動なので、平行移動の逆だけで足りる。
const jointY = [0, 0.45, 0.98, 1.28]
const ibm = []
for (const y of jointY) {
  ibm.push(
    1, 0, 0, 0,
    0, 1, 0, 0,
    0, 0, 1, 0,
    0, -y, 0, 1,
  )
}
const vIbm = push(Float32Array.from(ibm))

const bin = Buffer.concat(chunks)
const pm = minMax(positions)

const gltf = {
  asset: { version: '2.0', generator: 'soc-ai-agent placeholder-avatar' },
  scene: 0,
  scenes: [{ nodes: [0, 4] }],
  nodes: [
    // 0: Hips（ルート）
    { name: 'Hips', translation: [0, 0, 0], children: [1] },
    // 1: Spine
    { name: 'Spine', translation: [0, 0.45, 0], children: [2] },
    // 2: Neck
    { name: 'Neck', translation: [0, 0.53, 0], children: [3] },
    // 3: Head（うなずきはここを回す）
    { name: 'Head', translation: [0, 0.30, 0] },
    // 4: スキンメッシュ
    { name: 'Body', mesh: 0, skin: 0 },
  ],
  skins: [{ joints: [0, 1, 2, 3], inverseBindMatrices: 6, skeleton: 0 }],
  meshes: [{
    name: 'Body',
    primitives: [{
      attributes: { POSITION: 0, JOINTS_0: 2, WEIGHTS_0: 3 },
      indices: 1,
      material: 0,
      targets: [{ POSITION: 4 }, { POSITION: 5 }],
    }],
    // 表情名。avatar-capabilities.ts がこの名前を照合する。
    extras: { targetNames: ['blink', 'aa'] },
    weights: [0, 0],
  }],
  materials: [{
    name: 'Placeholder',
    pbrMetallicRoughness: { baseColorFactor: [0.72, 0.70, 0.66, 1], metallicFactor: 0, roughnessFactor: 0.8 },
  }],
  accessors: [
    { bufferView: 0, componentType: 5126, count: vertexCount, type: 'VEC3', min: pm.min, max: pm.max },
    { bufferView: 1, componentType: 5123, count: indices.length, type: 'SCALAR' },
    { bufferView: 2, componentType: 5123, count: vertexCount, type: 'VEC4' },
    { bufferView: 3, componentType: 5126, count: vertexCount, type: 'VEC4' },
    { bufferView: 4, componentType: 5126, count: vertexCount, type: 'VEC3', ...minMax(blinkDelta) },
    { bufferView: 5, componentType: 5126, count: vertexCount, type: 'VEC3', ...minMax(aaDelta) },
    { bufferView: 6, componentType: 5126, count: jointY.length, type: 'MAT4' },
  ],
  bufferViews: [vPos, vIdx, vJoint, vWeight, vBlink, vAa, vIbm].map((v) => ({
    buffer: 0, byteOffset: v.byteOffset, byteLength: v.byteLength,
  })),
  buffers: [{ byteLength: bin.length }],
}

// ─── glb にまとめる ─────────────────────────────────────────────────────────
const jsonBuf = Buffer.from(JSON.stringify(gltf), 'utf8')
const jsonPad = (4 - (jsonBuf.length % 4)) % 4
const jsonChunk = Buffer.concat([jsonBuf, Buffer.alloc(jsonPad, 0x20)])
const binPad = (4 - (bin.length % 4)) % 4
const binChunk = Buffer.concat([bin, Buffer.alloc(binPad)])

const header = Buffer.alloc(12)
header.writeUInt32LE(0x46546c67, 0)
header.writeUInt32LE(2, 4)
header.writeUInt32LE(12 + 8 + jsonChunk.length + 8 + binChunk.length, 8)

const jsonHeader = Buffer.alloc(8)
jsonHeader.writeUInt32LE(jsonChunk.length, 0)
jsonHeader.writeUInt32LE(0x4e4f534a, 4) // 'JSON'

const binHeader = Buffer.alloc(8)
binHeader.writeUInt32LE(binChunk.length, 0)
binHeader.writeUInt32LE(0x004e4942, 4) // 'BIN'

const glb = Buffer.concat([header, jsonHeader, jsonChunk, binHeader, binChunk])

for (const name of ['male', 'female']) {
  const out = `public/avatars/${name}-avatar-placeholder.glb`
  mkdirSync(dirname(out), { recursive: true })
  writeFileSync(out, glb)
  console.log(`${out} (${glb.length} bytes)`)
}
console.log('\n動作確認用の仮モデルです。見た目は箱なので、本番用は VRM に差し替えてください。')
console.log('npm run check:avatar で要件を満たしているか確認できます。')
