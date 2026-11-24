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
export const DEFAULT_SHADER_PATH = '/live2d/shaders/'

/**
 * 性別ごとのモデルURL。`NEXT_PUBLIC_LIVE2D_MODEL_URL` を明示指定した場合は
 * 性別によらずそれを使う（単一モデルでのローカル確認用、`public/live2d/README.md` 参照）。
 * 指定が無ければ `public/live2d/interviewers/{gender}/` の正規書き出し規約のパスを使う。
 */
export function modelUrlForGender(gender: 'male' | 'female'): string {
  return (
    process.env.NEXT_PUBLIC_LIVE2D_MODEL_URL ??
    `/live2d/interviewers/${gender}/${gender}-interviewer.model3.json`
  )
}

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

let preloadStarted = false

/**
 * 面接が始まる前に、Live2D の Core・Framework・モデル一式を先に読み始める。
 *
 * 面接官を表示するときに読み始めると、Core → Framework → model3.json → moc3 → テクスチャ
 * を1つずつ待つことになり、表示まで数秒かかる。企業を選んでいる間に読んでおけば、
 * 表示のときは読み込み済み（HTTP キャッシュ）になる。失敗は無視する（本番の読み込みで改めて扱う）。
 */
export function preloadLive2D(gender: 'male' | 'female'): void {
  if (preloadStarted || typeof window === 'undefined') return
  if (process.env.NEXT_PUBLIC_LIVE2D_ENABLED !== 'true') return
  preloadStarted = true

  // Framework は Core のグローバルを参照するので、Core の後に読む（Live2DAvatar と同じ順番）
  ensureCubismCore()
    .then(() => import('./live2d-framework-model'))
    .catch(() => {})

  const modelUrl = modelUrlForGender(gender)
  const dir = modelUrl.slice(0, modelUrl.lastIndexOf('/') + 1)
  fetch(modelUrl)
    .then((res) => (res.ok ? res.json() : null))
    .then((model: { FileReferences?: { Moc?: unknown; Physics?: unknown; Textures?: unknown } } | null) => {
      const refs = model?.FileReferences
      if (!refs) return
      const textures = Array.isArray(refs.Textures) ? refs.Textures : []
      for (const file of [refs.Moc, refs.Physics, ...textures]) {
        if (typeof file === 'string') void fetch(dir + file).catch(() => {})
      }
    })
    .catch(() => {})
}
