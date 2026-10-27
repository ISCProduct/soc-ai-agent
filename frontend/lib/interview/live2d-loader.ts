/**
 * Live2D Cubism Core の読み込みとモデルからの情報抽出。
 *
 * Core（`live2dcubismcore.*.js`）は Live2D Proprietary Software License の
 * バイナリなので npm には存在せず、リポジトリにも入れていない
 * （`.gitignore` の `frontend/public/live2d/` 参照）。live2d.com から Cubism SDK for Web
 * を取得して同意した人が `public/live2d/core/` に置く前提で、ここでは
 * <script> タグとして実行時に読み込むだけにする。
 *
 * モデル本体（.model3.json 一式）も配布条件がリポジトリと別なので同様に
 * 実行時に `public/live2d/model/` から読む。どちらも無ければ `loadLive2DModel`
 * が reject し、呼び出し側（Live2DAvatar）が既存の ThreeAvatar へフォールバックする
 * （描画は止めない。live2d-params.ts と同じ方針）。
 */
import type { Live2DModelInfo } from './live2d-params'

export const DEFAULT_CORE_URL =
  process.env.NEXT_PUBLIC_LIVE2D_CORE_URL ?? '/live2d/core/live2dcubismcore.min.js'
export const DEFAULT_MODEL_URL =
  process.env.NEXT_PUBLIC_LIVE2D_MODEL_URL ?? '/live2d/model/model.model3.json'

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

/**
 * pixi-live2d-display の `Live2DModel`（cubism4）インスタンスから、
 * `inspectLive2DModel` が要求する形に情報を抜き出す。
 *
 * モデルの実体にのみ依存する形にしてあるので、テストではこの形を満たす
 * 最小限のダミーを渡せば実際の pixi-live2d-display 無しで検証できる。
 */
export interface Live2DModelLike {
  internalModel: {
    coreModel: {
      getModel(): { parameters: { ids: readonly string[] } }
    }
    settings: {
      json?: {
        Groups?: ReadonlyArray<{ Name?: string; Ids?: readonly string[] }>
        FileReferences?: { Groups?: ReadonlyArray<{ Name?: string; Ids?: readonly string[] }> }
      }
    }
  }
}

function groupIds(model: Live2DModelLike, name: string): readonly string[] | undefined {
  const json = model.internalModel.settings.json
  const groups = json?.Groups ?? json?.FileReferences?.Groups ?? []
  return groups.find((g) => g.Name === name)?.Ids
}

export function toLive2DModelInfo(model: Live2DModelLike): Live2DModelInfo {
  return {
    parameterIds: model.internalModel.coreModel.getModel().parameters.ids,
    lipSyncIds: groupIds(model, 'LipSync'),
    eyeBlinkIds: groupIds(model, 'EyeBlink'),
  }
}
