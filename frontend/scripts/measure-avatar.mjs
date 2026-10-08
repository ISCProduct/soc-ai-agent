#!/usr/bin/env node
/**
 * アバター GLB を「実際にスキニングして」測る。
 *
 * うなずきが首かしげになっていた件を、骨の無い合成リグのテストは1件も
 * 捕まえられなかった。合成リグは正しかったが、シップするモデルは
 * 向き補正をジョイントの rotation に焼き込んでいて Head のローカル軸が
 * 回っていた。つまり「実物の幾何」と突き合わせないと分からない。
 *
 *   node scripts/measure-avatar.mjs                      # public/avatars の2体
 *   node scripts/measure-avatar.mjs path/to/model.glb
 *
 * 出力する数値は README の表の出どころ。モデルを作り直したら必ず再実行する。
 * tests/lib/interview/avatar-geometry.test.ts がこのモジュールを使って
 * 「うなずくと鼻が下がって前に出る」ことを固定している。
 */
import { readFileSync } from 'node:fs'
import { basename } from 'node:path'

const GLTF_MAGIC = 0x46546c67
const NUM = { SCALAR: 1, VEC2: 2, VEC3: 3, VEC4: 4, MAT4: 16 }
const BYTES = { 5120: 1, 5121: 1, 5122: 2, 5123: 2, 5125: 4, 5126: 4 }
const NORM_MAX = { 5120: 127, 5121: 255, 5122: 32767, 5123: 65535 }

export function readGlb(path) {
  const buf = readFileSync(path)
  if (buf.readUInt32LE(0) !== GLTF_MAGIC) throw new Error('glTF バイナリではありません')
  const jsonLen = buf.readUInt32LE(12)
  const json = JSON.parse(buf.subarray(20, 20 + jsonLen).toString('utf8'))
  const p = 20 + jsonLen
  return { json, bin: buf.subarray(p + 8, p + 8 + buf.readUInt32LE(p)), bytes: buf.length }
}

function readRaw(bin, componentType, off) {
  switch (componentType) {
    case 5126: return bin.readFloatLE(off)
    case 5125: return bin.readUInt32LE(off)
    case 5123: return bin.readUInt16LE(off)
    case 5122: return bin.readInt16LE(off)
    case 5121: return bin.readUInt8(off)
    case 5120: return bin.readInt8(off)
    default: throw new Error(`未対応の componentType: ${componentType}`)
  }
}

/** accessor を Float64Array で読む。byteStride と sparse に対応する。 */
export function readAccessor(json, bin, index) {
  const a = json.accessors[index]
  const n = NUM[a.type]
  const size = BYTES[a.componentType]
  const out = new Float64Array(a.count * n)
  if (a.bufferView !== undefined) {
    const bv = json.bufferViews[a.bufferView]
    // インターリーブされた GLB では stride を無視すると無言で壊れた値を読む
    const stride = bv.byteStride ?? n * size
    const start = (bv.byteOffset ?? 0) + (a.byteOffset ?? 0)
    for (let i = 0; i < a.count; i++) {
      for (let k = 0; k < n; k++) out[i * n + k] = readRaw(bin, a.componentType, start + i * stride + k * size)
    }
  }
  if (a.normalized && NORM_MAX[a.componentType]) {
    const m = NORM_MAX[a.componentType]
    for (let i = 0; i < out.length; i++) out[i] /= m
  }
  if (a.sparse) {
    // sparse の indices/values は accessor ではなく生の bufferView 指定なので、
    // 一時的な accessor を作って同じ経路で読む
    const idx = readAccessor(json, bin, pushTemp(json, a.sparse.indices, a.sparse.count, 'SCALAR'))
    const val = readAccessor(json, bin, pushTemp(json, a.sparse.values, a.sparse.count, a.type))
    json.accessors.length -= 2
    for (let i = 0; i < a.sparse.count; i++) {
      for (let k = 0; k < n; k++) out[idx[i] * n + k] = val[i * n + k]
    }
  }
  return out
}

function pushTemp(json, ref, count, type) {
  json.accessors.push({
    bufferView: ref.bufferView, byteOffset: ref.byteOffset ?? 0,
    componentType: ref.componentType ?? 5126, count, type,
  })
  return json.accessors.length - 1
}

// ─── 4x4（列優先、glTF と同じ） ──────────────────────────────────────────────
const ident = () => [1,0,0,0, 0,1,0,0, 0,0,1,0, 0,0,0,1]
function mul(a, b) {
  const o = new Array(16).fill(0)
  for (let c = 0; c < 4; c++) for (let r = 0; r < 4; r++) {
    let s = 0
    for (let k = 0; k < 4; k++) s += a[k * 4 + r] * b[c * 4 + k]
    o[c * 4 + r] = s
  }
  return o
}
function apply(m, x, y, z) {
  return [
    m[0] * x + m[4] * y + m[8] * z + m[12],
    m[1] * x + m[5] * y + m[9] * z + m[13],
    m[2] * x + m[6] * y + m[10] * z + m[14],
  ]
}
function nodeMatrix(node) {
  if (node.matrix) return node.matrix.slice()
  const t = node.translation ?? [0, 0, 0]
  const [x, y, z, w] = node.rotation ?? [0, 0, 0, 1]
  const s = node.scale ?? [1, 1, 1]
  const r = [
    1 - 2 * (y * y + z * z), 2 * (x * y + z * w), 2 * (x * z - y * w), 0,
    2 * (x * y - z * w), 1 - 2 * (x * x + z * z), 2 * (y * z + x * w), 0,
    2 * (x * z + y * w), 2 * (y * z - x * w), 1 - 2 * (x * x + y * y), 0,
    0, 0, 0, 1,
  ]
  for (let c = 0; c < 3; c++) for (let k = 0; k < 3; k++) r[c * 4 + k] *= s[c]
  r[12] = t[0]; r[13] = t[1]; r[14] = t[2]
  return r
}
function rotX(rad) {
  const c = Math.cos(rad), s = Math.sin(rad)
  return [1,0,0,0, 0,c,s,0, 0,-s,c,0, 0,0,0,1]
}

/**
 * スキニングして頂点のワールド座標を返す。
 *
 * three.js の SkinnedMesh と同じ式: Σ w_j (globalJoint_j × IBM_j) × p
 * extra に {ボーン名: ローカル回転行列} を渡すとその骨を回した結果になる。
 * スキンを持つノードの transform は glTF 仕様上無視されるので見ない。
 */
export function skinnedPositions({ json, bin }, { headRotX = 0, mouth = 0 } = {}) {
  const prim = json.meshes[0].primitives[0]
  const pos = readAccessor(json, bin, prim.attributes.POSITION)
  const count = pos.length / 3
  const morph = mouth !== 0 && prim.targets?.[0]?.POSITION !== undefined
    ? readAccessor(json, bin, prim.targets[0].POSITION)
    : null

  const skin = json.skins?.[0]
  if (!skin) throw new Error('skin がありません（骨格の無い静止メッシュ）')
  const ibm = readAccessor(json, bin, skin.inverseBindMatrices)

  // 親をたどってグローバル行列を作る
  const parent = new Map()
  json.nodes.forEach((n, i) => { for (const c of n.children ?? []) parent.set(c, i) })
  const extra = new Map()
  if (headRotX !== 0) {
    const head = json.nodes.findIndex((n) => n.name === 'Head')
    if (head < 0) throw new Error('Head ノードがありません')
    extra.set(head, rotX(headRotX))
  }
  const cache = new Map()
  const global = (i) => {
    if (cache.has(i)) return cache.get(i)
    let m = nodeMatrix(json.nodes[i])
    const e = extra.get(i)
    if (e) m = mul(m, e)
    const p = parent.get(i)
    const out = p === undefined ? m : mul(global(p), m)
    cache.set(i, out)
    return out
  }
  const skinMats = skin.joints.map((j, k) => mul(global(j), ibm.slice(k * 16, k * 16 + 16)))

  const joints = readAccessor(json, bin, prim.attributes.JOINTS_0)
  const weights = readAccessor(json, bin, prim.attributes.WEIGHTS_0)
  const out = new Float64Array(count * 3)
  for (let i = 0; i < count; i++) {
    let x = pos[i * 3], y = pos[i * 3 + 1], z = pos[i * 3 + 2]
    if (morph) { x += morph[i * 3] * mouth; y += morph[i * 3 + 1] * mouth; z += morph[i * 3 + 2] * mouth }
    let ax = 0, ay = 0, az = 0
    for (let k = 0; k < 4; k++) {
      const w = weights[i * 4 + k]
      if (w === 0) continue
      const [vx, vy, vz] = apply(skinMats[joints[i * 4 + k]], x, y, z)
      ax += vx * w; ay += vy * w; az += vz * w
    }
    out[i * 3] = ax; out[i * 3 + 1] = ay; out[i * 3 + 2] = az
  }
  return out
}

/** 鼻と頭頂の添字。鼻は「上部で最も前（+Z）に出ている頂点」。 */
export function landmarks(p) {
  const count = p.length / 3
  let yMin = Infinity, yMax = -Infinity
  for (let i = 0; i < count; i++) {
    const y = p[i * 3 + 1]
    if (y < yMin) yMin = y
    if (y > yMax) yMax = y
  }
  const top = yMin + (yMax - yMin) * 0.75
  let crown = 0, nose = -1, noseZ = -Infinity
  for (let i = 0; i < count; i++) {
    if (p[i * 3 + 1] > p[crown * 3 + 1]) crown = i
    if (p[i * 3 + 1] > top && p[i * 3 + 2] > noseZ) { noseZ = p[i * 3 + 2]; nose = i }
  }
  return { crown, nose, height: yMax - yMin }
}

const dv = (a, b, i) => [a[i * 3] - b[i * 3], a[i * 3 + 1] - b[i * 3 + 1], a[i * 3 + 2] - b[i * 3 + 2]]
const fmt = (v) => `(${v.map((n) => n.toFixed(4)).join(', ')})`

/**
 * 辺の伸縮。変形でメッシュが裂けていないかを見る。
 *
 * 伸縮率の最大値だけ見ると判断を誤る。このメッシュには長さ 2e-5（高さの
 * 0.002%）のような極小の辺が混ざっていて、そこは変位がごく僅かでも
 * 率が数百%になる。目に見える辺だけの最大値（maxVisible）を併せて出す。
 */
const VISIBLE_EDGE = 1e-3  // 高さ1.0に正規化したモデルでの「見える辺」の下限

function stretch(rest, moved, json, bin) {
  const prim = json.meshes[0].primitives[0]
  const idx = readAccessor(json, bin, prim.indices)
  const seen = new Set()
  let max = 0, maxLen = 0, maxVisible = 0, over10 = 0, over25 = 0, total = 0
  const len = (p, a, b) => Math.hypot(
    p[a * 3] - p[b * 3], p[a * 3 + 1] - p[b * 3 + 1], p[a * 3 + 2] - p[b * 3 + 2])
  for (let t = 0; t < idx.length; t += 3) {
    for (const [a, b] of [[0, 1], [1, 2], [2, 0]]) {
      const i = idx[t + a], j = idx[t + b]
      const key = i < j ? i * 1e7 + j : j * 1e7 + i
      if (seen.has(key)) continue
      seen.add(key)
      const l0 = len(rest, i, j)
      if (l0 === 0) continue
      const r = Math.abs(len(moved, i, j) - l0) / l0
      total++
      if (r > max) { max = r; maxLen = l0 }
      if (l0 >= VISIBLE_EDGE && r > maxVisible) maxVisible = r
      if (r > 0.10) over10++
      if (r > 0.25) over25++
    }
  }
  return { max, maxLen, maxVisible, over10, over25, total }
}

export function measure(path, { nod = 0.11 } = {}) {
  const glb = readGlb(path)
  const rest = skinnedPositions(glb)
  const nodded = skinnedPositions(glb, { headRotX: nod })
  const opened = skinnedPositions(glb, { mouth: 1 })
  const lm = landmarks(rest)
  return {
    name: basename(path),
    mb: glb.bytes / 1024 / 1024,
    height: lm.height,
    noseZ: rest[lm.nose * 3 + 2],
    nodNose: dv(nodded, rest, lm.nose),
    nodCrown: dv(nodded, rest, lm.crown),
    nodStretch: stretch(rest, nodded, glb.json, glb.bin),
    mouthStretch: stretch(rest, opened, glb.json, glb.bin),
    mouthMoved: (() => {
      let n = 0
      for (let i = 0; i < rest.length / 3; i++) if (Math.hypot(...dv(opened, rest, i)) > 1e-6) n++
      return n
    })(),
  }
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const targets = process.argv.slice(2).filter((a) => !a.startsWith('--'))
  const files = targets.length > 0 ? targets
    : ['public/avatars/male-avatar.glb', 'public/avatars/female-avatar.glb']
  for (const f of files) {
    const m = measure(f)
    console.log(`\n${m.name}  ${m.mb.toFixed(1)}MB  高さ ${m.height.toFixed(3)}`)
    console.log(`  最前方(上部)頂点 Z = ${m.noseZ.toFixed(3)}（顔が +Z を向いている）`)
    console.log(`  うなずき 6.3度  鼻 ${fmt(m.nodNose)}  頭頂 ${fmt(m.nodCrown)}`)
    console.log(`                  → 鼻が ${m.nodNose[1] < 0 ? '下がり' : '上がり'}` +
      `${m.nodNose[2] > 0 ? '前に出る' : '後ろへ引く'}`)
    for (const [label, s] of [['うなずき 6.3度', m.nodStretch], ['口 全開', m.mouthStretch]]) {
      console.log(`  ${label} の辺の伸縮  最大 ${(s.max * 100).toFixed(2)}%` +
        `（その辺の長さ ${s.maxLen.toExponential(1)}）` +
        `  見える辺の最大 ${(s.maxVisible * 100).toFixed(2)}%`)
      console.log(`    10%超 ${s.over10.toLocaleString()} / ${s.total.toLocaleString()}` +
        `  25%超 ${s.over25.toLocaleString()}`)
    }
    console.log(`  口で動く頂点 ${m.mouthMoved.toLocaleString()}`)
  }
}
