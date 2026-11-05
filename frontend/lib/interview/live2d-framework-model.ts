/**
 * Live2D公式Cubism Web Framework（`vendor/live2d-framework/`）を直接使ったモデルのロードと描画。
 *
 * 旧実装は `pixi-live2d-display`（2023年公開、以後更新なし）を使っていたが、
 * 現行配布のCore（Core version 06.00.0001, 2026年以降の「Cubism 5 SDK」）と
 * API非互換（`drawables.renderOrders`が`drawOrders`に改名、他）があり、
 * パッチしても描画が崩れた。公式Frameworkは現行Coreと同じベンダーが
 * メンテしているため非互換が起きない。
 */
import { CubismFramework } from '@/vendor/live2d-framework/src/live2dcubismframework'
import { CubismModelSettingJson } from '@/vendor/live2d-framework/src/cubismmodelsettingjson'
import type { ICubismModelSetting } from '@/vendor/live2d-framework/src/icubismmodelsetting'
import { CubismUserModel } from '@/vendor/live2d-framework/src/model/cubismusermodel'
import { CubismMatrix44 } from '@/vendor/live2d-framework/src/math/cubismmatrix44'
import { buildCapabilitiesInfo, type Live2DFrameworkCapabilitiesInfo } from './live2d-capabilities-info'
import { OVERLAY_MESH_IDS, overlayOpacities, type OverlayMeshIds } from './live2d-overlay'
import { verticesExtent } from './live2d-extent'

export type { Live2DFrameworkCapabilitiesInfo }

let frameworkStarted = false

/** Cubism Framework全体の初期化。プロセス内で1度だけでよい。 */
export function ensureCubismFramework(): void {
  if (frameworkStarted) return
  CubismFramework.startUp()
  CubismFramework.initialize()
  frameworkStarted = true
}

async function fetchArrayBuffer(url: string): Promise<ArrayBuffer> {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`${url} の取得に失敗した (HTTP ${res.status})`)
  return res.arrayBuffer()
}

function loadTexture(gl: WebGLRenderingContext, url: string): Promise<WebGLTexture> {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.onload = () => {
      const tex = gl.createTexture()
      if (!tex) {
        reject(new Error(`${url} 用のWebGLTextureを作成できなかった`))
        return
      }
      gl.bindTexture(gl.TEXTURE_2D, tex)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
      gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, true)
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, img)
      gl.generateMipmap(gl.TEXTURE_2D)
      gl.bindTexture(gl.TEXTURE_2D, null)
      resolve(tex)
    }
    img.onerror = () => reject(new Error(`${url} の読み込みに失敗した`))
    img.src = url
  })
}

/** モデル1体ぶんのロード・パラメータ設定・描画をまとめたラッパー。 */
export class Live2DFrameworkModel extends CubismUserModel {
  private modelSetting: ICubismModelSetting | null = null
  private readonly viewMatrix = new CubismMatrix44()
  private overlay = { eyeOpen: 1, mouthOpen: 0 }
  private overlayIndices: { base: number | null; eyesClosed: number; mouthOpen: number } | null = null
  private measuredExtent: { width: number; height: number } | null = null

  static async load(
    gl: WebGLRenderingContext,
    modelDir: string,
    modelFileName: string,
    shaderPath: string,
    initialWidthPx: number,
    initialHeightPx: number,
    overlayMeshIds: OverlayMeshIds = OVERLAY_MESH_IDS,
  ): Promise<Live2DFrameworkModel> {
    const model = new Live2DFrameworkModel()

    const settingBuffer = await fetchArrayBuffer(`${modelDir}${modelFileName}`)
    const setting: ICubismModelSetting = new CubismModelSettingJson(settingBuffer, settingBuffer.byteLength)
    model.modelSetting = setting

    const mocFileName = setting.getModelFileName()
    if (!mocFileName) throw new Error('model3.json にMocファイルの指定が無い')
    const mocBuffer = await fetchArrayBuffer(`${modelDir}${mocFileName}`)
    model.loadModel(mocBuffer)
    if (!model.getModel()) throw new Error('moc3の読み込みに失敗した（壊れているか非対応バージョン）')

    // ここから先で投げると、直前に読んだmocのネイティブメモリが解放されないまま残る
    // （面接画面を開き直すたびに増える）。失敗時は必ず release してから投げ直す。
    try {
      const drawableIds: string[] = Array.from(model.getModel().getModel().drawables.ids)
      const eyesClosed = drawableIds.indexOf(overlayMeshIds.eyesClosed)
      const mouthOpen = drawableIds.indexOf(overlayMeshIds.mouthOpen)
      const base = drawableIds.indexOf(overlayMeshIds.base)
      if (eyesClosed >= 0 && mouthOpen >= 0) {
        model.overlayIndices = { base: base >= 0 ? base : null, eyesClosed, mouthOpen }
      }

      const physicsFileName = setting.getPhysicsFileName()
      if (physicsFileName) {
        // physicsは任意（READMEの必須成果物ではない）。取得できないときは揺れなしで描画を続ける。
        // ここで投げるとテクスチャが揃っていてもAvatar2Dへ落ちてしまう。
        try {
          const physicsBuffer = await fetchArrayBuffer(`${modelDir}${physicsFileName}`)
          model.loadPhysics(physicsBuffer, physicsBuffer.byteLength)
        } catch (err) {
          console.warn(`[Live2D] ${physicsFileName} を読み込めなかったので揺れなしで描画する`, err)
        }
      }

      model.createRenderer(Math.max(1, initialWidthPx), Math.max(1, initialHeightPx))
      const renderer = model.getRenderer()
      renderer.startUp(gl)
      renderer.loadShaders(shaderPath)
      renderer.setIsPremultipliedAlpha(true)

      const textureCount = setting.getTextureCount()
      for (let i = 0; i < textureCount; i++) {
        const textureFileName = setting.getTextureFileName(i)
        if (!textureFileName) continue
        const texture = await loadTexture(gl, `${modelDir}${textureFileName}`)
        renderer.bindTexture(i, texture)
      }

      return model
    } catch (err) {
      model.release()
      throw err
    }
  }

  /** 面接官として使えるか判定するための情報（`live2d-params.ts`の`inspectLive2DModel`にそのまま渡せる形）。 */
  getCapabilitiesInfo(): Live2DFrameworkCapabilitiesInfo {
    const setting = this.modelSetting
    return buildCapabilitiesInfo(
      this.getModel().getModel().parameters.ids,
      setting ? { count: setting.getLipSyncParameterCount(), idAt: (i) => setting.getLipSyncParameterId(i).getString() } : null,
      setting ? { count: setting.getEyeBlinkParameterCount(), idAt: (i) => setting.getEyeBlinkParameterId(i).getString() } : null,
    )
  }

  resize(widthPx: number, heightPx: number): void {
    this.setRenderTargetSize(Math.max(1, widthPx), Math.max(1, heightPx))
  }

  setParameterValue(id: string, value: number): void {
    this.getModel().setParameterValueById(CubismFramework.getIdManager().getId(id), value)
  }

  /**
   * 目の開き・口の開き(0-1)を差分ArtMeshの不透明度として次の `commitFrame` で反映する。
   * 差分ArtMeshがモデルに無ければ何もしない。`live2d-overlay.ts` 先頭のコメント参照。
   */
  setOverlayState(eyeOpen: number, mouthOpen: number): void {
    this.overlay = { eyeOpen, mouthOpen }
  }

  /** 差分ArtMeshを持つモデルか（書き出し済みの男性・女性モデルはいずれも true）。 */
  hasOverlayMeshes(): boolean {
    return this.overlayIndices !== null
  }

  /** 1フレームぶんパラメータを確定させてCoreのdrawable頂点を再計算する。描画前に呼ぶ。 */
  commitFrame(deltaTimeSeconds: number): void {
    const model = this.getModel()
    model.saveParameters()
    this._physics?.evaluate(model, deltaTimeSeconds)
    model.update()
    this.applyOverlayOpacities()
  }

  private applyOverlayOpacities(): void {
    const indices = this.overlayIndices
    if (!indices) return
    const { eyesClosed, mouthOpen } = overlayOpacities(this.overlay.eyeOpen, this.overlay.mouthOpen)
    // Core の drawables.opacities は描画側(renderer)が読むのと同じ配列。update() のたびに
    // Core が計算し直すので、毎フレームこの直後に上書きする。
    const opacities = this.getModel().getModel().drawables.opacities
    opacities[indices.eyesClosed] = eyesClosed
    opacities[indices.mouthOpen] = mouthOpen
    // 不透明度とは別に、Coreが計算する「IsVisible」動的フラグ(bit 0)が false だと
    // レンダラーは不透明度を見ずにその drawable の描画自体をスキップする
    // (`cubismrenderer_webgl.ts:973` が `getDrawableDynamicFlagIsVisible` で弾く。
    //  判定は Core の `hasIsVisibleBit` で bit 0)。
    // 不透明度を上書きするなら、このフラグも揃えないと効かない。
    const dynamicFlags = this.getModel().getModel().drawables.dynamicFlags
    dynamicFlags[indices.eyesClosed] =
      eyesClosed > 0 ? dynamicFlags[indices.eyesClosed] | 1 : dynamicFlags[indices.eyesClosed] & ~1
    dynamicFlags[indices.mouthOpen] =
      mouthOpen > 0 ? dynamicFlags[indices.mouthOpen] | 1 : dynamicFlags[indices.mouthOpen] & ~1
    // 書き出しによっては通常絵(base)のCore側の既定値が不透明度0・不可視になっている
    // （女性モデルで確認。これが原因で画面に何も出なかった）。差分2枚以外の常時表示
    // レイヤーは、Coreの計算値に関わらず毎フレーム見える状態に固定する
    // （男性モデルでは既にそうなっているので上書きしても無害）。
    if (indices.base !== null) {
      opacities[indices.base] = 1
      dynamicFlags[indices.base] |= 1
    }
  }

  /**
   * 全drawableの頂点から、実際の絵の見た目サイズ(unit空間)を測る。
   *
   * `getCanvasWidth()/getCanvasHeight()` は Cubism Editor のキャンバス設定値をそのまま
   * 返すので、書き出しによっては実際の絵のサイズと食い違う（女性モデルで確認: 申告値が
   * 実寸の約0.6倍しかなく、拡大されて画面外へはみ出し何も映らなかった）。見た目の
   * スケールは実測した頂点の外接矩形から出す方が、Editor側の設定に左右されず安全。
   *
   * 頂点は update() のたびに変わるが、全身の外接矩形は口や目の開閉では実質変わらない。
   * 毎フレーム全頂点を走るのは無駄なので初回だけ測って使い回す。
   */
  private measureExtent(): { width: number; height: number } {
    if (this.measuredExtent) return this.measuredExtent
    const model = this.getModel()
    const vertexArrays: ArrayLike<number>[] = []
    const drawableCount = model.getDrawableCount()
    for (let i = 0; i < drawableCount; i++) vertexArrays.push(model.getDrawableVertices(i))
    this.measuredExtent = verticesExtent(vertexArrays)
    return this.measuredExtent
  }

  /**
   * canvasのピクセルサイズいっぱいに、アスペクト比を保ったまま("contain")収まるスケールで描画する。
   * 「1 unit が何pxか」を幅・高さそれぞれで求めて小さい方を採用する（`measureExtent`参照）。
   *
   * `zoom` > 1 で拡大し、`focusY`（モデル画像の上端を0・下端を1とした位置）が
   * canvasの縦中央に来るようにずらす。全身絵の余白を切って顔を大きく見せるのに使う。
   */
  draw(gl: WebGLRenderingContext, canvasWidthPx: number, canvasHeightPx: number, margin = 0.98, zoom = 1, focusY = 0.5): void {
    const { width: canvasWidthUnit, height: canvasHeightUnit } = this.measureExtent()
    if (canvasWidthUnit <= 0 || canvasHeightUnit <= 0) return

    const pxPerUnit = Math.min(canvasWidthPx / canvasWidthUnit, canvasHeightPx / canvasHeightUnit) * margin * zoom
    const scaleX = (pxPerUnit * 2) / canvasWidthPx
    const scaleY = (pxPerUnit * 2) / canvasHeightPx

    this.viewMatrix.loadIdentity()
    this.viewMatrix.scale(scaleX, scaleY)
    // 画像中心(unit原点)から focusY の位置までの距離ぶん、上へ寄せる(clip空間は上がプラス)。
    this.viewMatrix.translate(0, (focusY - 0.5) * canvasHeightUnit * scaleY)

    const renderer = this.getRenderer()
    renderer.setMvpMatrix(this.viewMatrix)
    renderer.setRenderState(gl.getParameter(gl.FRAMEBUFFER_BINDING) as WebGLFramebuffer, [0, 0, canvasWidthPx, canvasHeightPx])
    renderer.drawModel()
  }
}
