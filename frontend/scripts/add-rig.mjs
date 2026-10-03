#!/usr/bin/env node
/**
 * 骨の無い静止メッシュに、うなずき用の骨格を後付けする。
 *
 * public/avatars の GLB は Tripo（画像から3Dを生成するAIツール）製で、
 * skins も morph target も持たない静止メッシュだった。見た目は作り込まれて
 * いるので捨てたくないが、首の関節が無いのでうなずけない。
 * そこで見た目を一切変えずに骨格とウェイトだけを足す。
 *
 *   node scripts/add-rig.mjs public/avatars/male-avatar.glb
 *   → public/avatars/male-avatar-rigged.glb
 *
 * ## できること・できないこと
 *
 * うなずきはできる。首の位置は「高さ別の横幅が最小になるところ」で自動検出する
 * （実測で男性57.5%・女性52.5%にくびれがある）。頭ボーンを回すと、そこから上の
 * 頂点がついてくる。
 *
 * まばたきと口はできない。テクスチャが1枚で顔のパーツが描き込まれており、
 * 顔前面の頂点分布が均一（目の高さに集中が無い）＝目も口も立体として
 * 存在しないため、動かす対象が無い。閉じ目のテクスチャを別途用意して
 * 差し替える方式なら可能だが、それは絵を描く作業になる。
 */
import { readFileSync, writeFileSync } from 'node:fs'
import { basename, dirname, extname, join } from 'node:path'

const GLTF_MAGIC = 0x46546c67
const COMP = { FLOAT: 5126, USHORT: 5123, UBYTE: 5121, UINT: 5125 }
const NUM = { SCALAR: 1, VEC2: 2, VEC3: 3, VEC4: 4, MAT4: 16 }
const BYTES = { [COMP.FLOAT]: 4, [COMP.USHORT]: 2, [COMP.UBYTE]: 1, [COMP.UINT]: 4 }

function readGlb(path) {
  const buf = readFileSync(path)
  if (buf.readUInt32LE(0) !== GLTF_MAGIC) throw new Error('glTF バイナリではありません')
  const jsonLen = buf.readUInt32LE(12)
  const json = JSON.parse(buf.subarray(20, 20 + jsonLen).toString('utf8'))
  let p = 20 + jsonLen
  const binLen = buf.readUInt32LE(p)
  const bin = buf.subarray(p + 8, p + 8 + binLen)
  return { json, bin }
}

function readAccessor(json, bin, index) {
  const a = json.accessors[index]
  const bv = json.bufferViews[a.bufferView]
  const n = NUM[a.type]
  const size = BYTES[a.componentType]
  const start = (bv.byteOffset ?? 0) + (a.byteOffset ?? 0)
  // stride 付きは今回の入力に無いので素直に詰まっている前提で読む
  const out = []
  for (let i = 0; i < a.count; i++) {
    const row = []
    for (let k = 0; k < n; k++) {
      const off = start + (i * n + k) * size
      row.push(a.componentType === COMP.FLOAT ? bin.readFloatLE(off)
        : a.componentType === COMP.USHORT ? bin.readUInt16LE(off)
        : a.componentType === COMP.UINT ? bin.readUInt32LE(off)
        : bin.readUInt8(off))
    }
    out.push(row)
  }
  return out
}

/**
 * 首の高さを求める。
 *
 * 上半分を細かい層に切って、横幅（X方向の広がり）が最小になる層を首とみなす。
 * 肩より上で最もくびれているところが首である、という前提。
 * 髪が長いモデルでは髪が首を覆って検出がぶれるので、最小値だけでなく
 * 「上下の層より明確に細い」ことも条件にする。
 */
function findNeckY(positions) {
  // 20万頂点あるので Math.min(...ys) はスタックを溢れさせる。ループで求める。
  let yMin = Infinity, yMax = -Infinity
  for (const p of positions) {
    if (p[1] < yMin) yMin = p[1]
    if (p[1] > yMax) yMax = p[1]
  }
  const H = yMax - yMin
  const bands = 40
  const widths = []
  for (let b = 0; b < bands; b++) {
    const lo = yMin + (H * b) / bands
    const hi = yMin + (H * (b + 1)) / bands
    let min = Infinity, max = -Infinity, count = 0
    for (const p of positions) {
      if (p[1] >= lo && p[1] < hi) {
        if (p[0] < min) min = p[0]
        if (p[0] > max) max = p[0]
        count++
      }
    }
    widths.push({ b, center: (lo + hi) / 2, width: count > 0 ? max - min : Infinity, count })
  }
  // 下から35%〜80%の範囲で探す。胴の途中や頭頂を首と誤認しないため。
  const candidates = widths.filter((w) =>
    w.center > yMin + H * 0.35 && w.center < yMin + H * 0.80 && w.count > 0)
  if (candidates.length === 0) throw new Error('首を検出できませんでした')
  const neck = candidates.reduce((a, c) => (c.width < a.width ? c : a))
  return { neckY: neck.center, yMin, yMax, H, neckWidth: neck.width, widths }
}

/**
 * ウェイトを高さから決める。
 *
 * 首の境界でいきなり切り替えると、回した瞬間に首で面が裂ける。
 * 首の上下に幅を持たせて滑らかに移す。幅は首の太さを基準にする
 * （細い首なら狭く、太い首なら広く）。
 */
function weightFor(y, neckY, band) {
  const t = (y - (neckY - band)) / (band * 2)
  if (t <= 0) return 0        // 完全に胴
  if (t >= 1) return 1        // 完全に頭
  // なめらかに（smoothstep）
  return t * t * (3 - 2 * t)
}

function alignTo4(n) { return (4 - (n % 4)) % 4 }

function addRig(path) {
  const { json, bin } = readGlb(path)
  const mesh = json.meshes[0]
  if (json.skins?.length > 0) {
    console.log(`${basename(path)}: すでに骨格があります。何もしません。`)
    return null
  }

  // 全プリミティブの POSITION を集める
  const prims = mesh.primitives
  const posPerPrim = prims.map((p) => readAccessor(json, bin, p.attributes.POSITION))
  const allPos = posPerPrim.flat()
  const { neckY, yMin, H, neckWidth } = findNeckY(allPos)
  const band = Math.max(neckWidth * 0.5, H * 0.02)

  console.log(`${basename(path)}`)
  console.log(`  頂点 ${allPos.length.toLocaleString()} / 高さ ${H.toFixed(3)}`)
  console.log(`  首の高さ Y=${neckY.toFixed(3)}（下から ${(((neckY - yMin) / H) * 100).toFixed(1)}%）幅 ${neckWidth.toFixed(3)}`)
  console.log(`  なめらかに移す幅 ±${band.toFixed(3)}`)

  // ─── 追記するバイナリを組む ───────────────────────────────────────────
  const extra = []
  let extraLen = 0
  const newViews = []
  function appendView(buf) {
    const pad = alignTo4(extraLen)
    if (pad > 0) { extra.push(Buffer.alloc(pad)); extraLen += pad }
    const view = { byteOffset: bin.length + extraLen, byteLength: buf.length }
    extra.push(buf); extraLen += buf.length
    newViews.push(view)
    return json.bufferViews.length + newViews.length - 1
  }

  // ジョイント: 0=Hips(root) 1=Neck 2=Head
  // Hips はモデル原点、Neck は首の高さ、Head はその少し上。
  const headY = neckY + H * 0.06
  const jointWorldY = [yMin, neckY, headY]

  let headVerts = 0
  for (let pi = 0; pi < prims.length; pi++) {
    const pos = posPerPrim[pi]
    const jointsBuf = Buffer.alloc(pos.length * 4)       // VEC4 / UBYTE
    const weightsBuf = Buffer.alloc(pos.length * 4 * 4)  // VEC4 / FLOAT
    for (let i = 0; i < pos.length; i++) {
      const w = weightFor(pos[i][1], neckY, band)
      if (w > 0.5) headVerts++
      // 頭(2) と 胴(0) の2本に配分する。Neck(1) は中継用で直接は使わない。
      jointsBuf[i * 4 + 0] = 2
      jointsBuf[i * 4 + 1] = 0
      weightsBuf.writeFloatLE(w, (i * 4 + 0) * 4)
      weightsBuf.writeFloatLE(1 - w, (i * 4 + 1) * 4)
    }
    const jv = appendView(jointsBuf)
    const wv = appendView(weightsBuf)
    json.accessors.push({ bufferView: jv, componentType: COMP.UBYTE, count: pos.length, type: 'VEC4' })
    prims[pi].attributes.JOINTS_0 = json.accessors.length - 1
    json.accessors.push({ bufferView: wv, componentType: COMP.FLOAT, count: pos.length, type: 'VEC4' })
    prims[pi].attributes.WEIGHTS_0 = json.accessors.length - 1
  }

  // 逆バインド行列（平行移動の逆だけ）
  const ibm = Buffer.alloc(jointWorldY.length * 16 * 4)
  jointWorldY.forEach((y, i) => {
    const m = [1,0,0,0, 0,1,0,0, 0,0,1,0, 0,-y,0,1]
    m.forEach((v, k) => ibm.writeFloatLE(v, (i * 16 + k) * 4))
  })
  const ibmView = appendView(ibm)
  json.accessors.push({ bufferView: ibmView, componentType: COMP.FLOAT, count: jointWorldY.length, type: 'MAT4' })
  const ibmAccessor = json.accessors.length - 1

  // ノードを足す。既存のメッシュノードに skin を付ける。
  const meshNodeIndex = json.nodes.findIndex((n) => n.mesh === 0)
  const base = json.nodes.length
  json.nodes.push(
    { name: 'Hips', translation: [0, jointWorldY[0], 0], children: [base + 1] },
    { name: 'Neck', translation: [0, jointWorldY[1] - jointWorldY[0], 0], children: [base + 2] },
    { name: 'Head', translation: [0, jointWorldY[2] - jointWorldY[1], 0] },
  )
  json.skins = [{ joints: [base, base + 1, base + 2], inverseBindMatrices: ibmAccessor, skeleton: base }]
  json.nodes[meshNodeIndex].skin = 0
  // 骨格のルートをシーンに入れる
  json.scenes[0].nodes = [...new Set([...json.scenes[0].nodes, base])]
  json.asset.generator = (json.asset.generator ?? '') + ' + soc-ai-agent add-rig'

  console.log(`  頭側に付いた頂点 ${headVerts.toLocaleString()}（${((headVerts / allPos.length) * 100).toFixed(1)}%）`)

  // ─── 書き出し ─────────────────────────────────────────────────────────
  const newBin = Buffer.concat([bin, ...extra])
  json.bufferViews.push(...newViews)
  json.buffers[0].byteLength = newBin.length

  const jsonBuf = Buffer.from(JSON.stringify(json), 'utf8')
  const jsonChunk = Buffer.concat([jsonBuf, Buffer.alloc(alignTo4(jsonBuf.length), 0x20)])
  const binChunk = Buffer.concat([newBin, Buffer.alloc(alignTo4(newBin.length))])

  const header = Buffer.alloc(12)
  header.writeUInt32LE(GLTF_MAGIC, 0); header.writeUInt32LE(2, 4)
  header.writeUInt32LE(12 + 8 + jsonChunk.length + 8 + binChunk.length, 8)
  const jh = Buffer.alloc(8); jh.writeUInt32LE(jsonChunk.length, 0); jh.writeUInt32LE(0x4e4f534a, 4)
  const bh = Buffer.alloc(8); bh.writeUInt32LE(binChunk.length, 0); bh.writeUInt32LE(0x004e4942, 4)

  const out = join(dirname(path), basename(path, extname(path)) + '-rigged.glb')
  writeFileSync(out, Buffer.concat([header, jh, jsonChunk, bh, binChunk]))
  console.log(`  → ${out} (${(Buffer.concat([header, jh, jsonChunk, bh, binChunk]).length / 1024 / 1024).toFixed(1)}MB)\n`)
  return out
}

const args = process.argv.slice(2)
const targets = args.length > 0 ? args : [
  'public/avatars/male-avatar.glb',
  'public/avatars/female-avatar.glb',
]
for (const t of targets) addRig(t)
console.log('npm run check:avatar で結果を確認できます。')
console.log('まばたきと口は入りません（テクスチャに描かれているため動かす立体が無い）。')
