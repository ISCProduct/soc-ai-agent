#!/usr/bin/env node
/**
 * アバターを実際に描画してスクリーンショットを撮る。
 *
 * モデルを加工したら必ずこれで見ること。数値の検証だけでは足りない。
 * 実際に、骨を足したら ThreeAvatar の向き補正（hasSkeleton ? 0 : -PI/2）が
 * 効かなくなって横を向いた件と、口のモーフが顔ではなく側頭部を動かしていた件は、
 * どちらも描画して初めて気付いた。
 *
 *   # 静的サーバを立てる（frontend 直下で）
 *   python3 -m http.server 8099 --directory .
 *   node scripts/shoot-avatar.mjs <出力先ディレクトリ>
 */
import { chromium } from '@playwright/test'
import { mkdirSync } from 'node:fs'

const outDir = process.argv[2] ?? '/tmp/avatar-shots'
mkdirSync(outDir, { recursive: true })

// うなずきの向きと口の開きは全身の絵では小さすぎて判断できないので、
// 顔に寄せた絵（cam=face）も撮る。首かしげとうなずきの取り違えは
// 全身の絵では気付けなかった。
const POSES = [
  ['neutral', 0, 0],
  ['nod', 0.11, 0],   // AvatarMotion の nodDepthRad と同じ 6.3度
  ['mouth', 0, 1],    // 口を全開（音声振幅 1.0 のとき）
]
const shots = []
for (const g of ['male', 'female']) {
  for (const [pose, nod, mouth] of POSES) {
    for (const cam of ['', 'face']) {
      shots.push([`${g}-${pose}${cam ? '-face' : ''}`, `/public/avatars/${g}-avatar.glb`, nod, mouth, cam])
    }
  }
}

const browser = await chromium.launch({
  args: ['--use-gl=angle', '--use-angle=swiftshader', '--enable-unsafe-swiftshader'],
})
const page = await browser.newPage({ viewport: { width: 520, height: 620 }, deviceScaleFactor: 2 })
let failed = 0
for (const [name, file, nod, mouth, cam] of shots) {
  const url = `http://127.0.0.1:8099/scripts/avatar-preview.html?file=${encodeURIComponent(file)}&nod=${nod}&mouth=${mouth}&cam=${cam}`
  await page.goto(url, { waitUntil: 'load' })
  await page.waitForFunction('window.__ready === true', { timeout: 60000 })
  const err = await page.evaluate('window.__error')
  if (err) { console.log(`✗ ${name}: ${err}`); failed++; continue }
  const info = await page.evaluate('window.__info')
  await page.locator('#c').screenshot({ path: `${outDir}/${name}.png` })
  console.log(`✓ ${name}  頭ボーン=${info.headFound} 口モーフ=${info.morphFound}`)
}
await browser.close()
console.log(`\n${outDir} に出力しました。目で見て確認すること。`)
process.exit(failed > 0 ? 1 : 0)
