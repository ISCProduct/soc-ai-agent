/**
 * Live2D Cubism Core（`<script>`タグでの読み込み）とデフォルトパスの定義。
 *
 * Core（`live2dcubismcore.*.js`）は Live2D Proprietary Software License の
 * バイナリなので npm には存在せず、リポジトリにも入れていない
 * （`.gitignore` の `frontend/public/live2d/` 参照）。live2d.com から Cubism SDK for Web
 * を取得して同意した人が `public/live2d/core/` に置く前提で、ここでは
 * <script> タグとして実行時に読み込むだけにする。
 *
 * モデル本体（.model3.json 一式）も配布条件がリポジトリと別なので同様に
 * 実行時に `public/live2d/model/` から読む。どちらも無ければ Live2DAvatar が
 * 既存の Avatar2D へフォールバックする（描画は止めない。live2d-params.ts と同じ方針）。
 *
 * モデルの読み込み・描画自体は `live2d-framework-model.ts`（公式Cubism Web Framework
 * を直接使う実装）が担う。ここは「Coreスクリプトの注入」専任。
 */

export const DEFAULT_CORE_URL =
  process.env.NEXT_PUBLIC_LIVE2D_CORE_URL ?? '/live2d/core/live2dcubismcore.min.js'
export const DEFAULT_MODEL_URL =
  process.env.NEXT_PUBLIC_LIVE2D_MODEL_URL ?? '/live2d/model/model.model3.json'
export const DEFAULT_SHADER_PATH = '/live2d/shaders/'

let coreLoadPromise: Promise<void> | null = null

/** Core スクリプトを1度だけ注入する。既に `window.Live2DCubismCore` があれば何もしない。 */
export function ensureCubismCore(url: string = DEFAULT_CORE_URL): Promise<void> {
  if (typeof window === 'undefined') {
    return Promise.reject(new Error('ensureCubismCore はブラウザでのみ呼べる'))
  }
  if ((window as unknown as { Live2DCubismCore?: unknown }).Live2DCubismCore) {
    return Promise.resolve()
  }
  if (coreLoadPromise) return coreLoadPromise

  coreLoadPromise = new Promise<void>((resolve, reject) => {
    const script = document.createElement('script')
    script.src = url
    script.async = true
    script.onload = () => {
      if ((window as unknown as { Live2DCubismCore?: unknown }).Live2DCubismCore) {
        resolve()
      } else {
        reject(new Error(`${url} は読み込めたが Live2DCubismCore が定義されなかった`))
      }
    }
    script.onerror = () => reject(new Error(`${url} の読み込みに失敗した（未配置の可能性）`))
    document.head.appendChild(script)
  }).catch((err) => {
    coreLoadPromise = null // 失敗時は次回また試せるようにする（モデル差し替え後の再読込等）
    throw err
  })

  return coreLoadPromise
}
