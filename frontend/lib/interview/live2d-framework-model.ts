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

  static async load(
    gl: WebGLRenderingContext,
    modelDir: string,
    modelFileName: string,
    shaderPath: string,
    initialWidthPx: number,
    initialHeightPx: number,
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

    const physicsFileName = setting.getPhysicsFileName()
    if (physicsFileName) {
      const physicsBuffer = await fetchArrayBuffer(`${modelDir}${physicsFileName}`)
      model.loadPhysics(physicsBuffer, physicsBuffer.byteLength)
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

  /** 1フレームぶんパラメータを確定させてCoreのdrawable頂点を再計算する。描画前に呼ぶ。 */
  commitFrame(deltaTimeSeconds: number): void {
    const model = this.getModel()
    model.saveParameters()
    this._physics?.evaluate(model, deltaTimeSeconds)
    model.update()
  }

  /**
   * canvasのピクセルサイズいっぱいに、アスペクト比を保ったまま("contain")収まるスケールで描画する。
   * Cubismのunit空間はCore依存（CanvasWidth/Height ÷ PixelsPerUnit）なので、
   * 「1 unit が何pxか」を幅・高さそれぞれで求めて小さい方を採用する。
   */
  draw(gl: WebGLRenderingContext, canvasWidthPx: number, canvasHeightPx: number, margin = 0.98): void {
    const model = this.getModel()
    const canvasWidthUnit = model.getCanvasWidth()
    const canvasHeightUnit = model.getCanvasHeight()
    if (canvasWidthUnit <= 0 || canvasHeightUnit <= 0) return

    const pxPerUnit = Math.min(canvasWidthPx / canvasWidthUnit, canvasHeightPx / canvasHeightUnit) * margin
    const scaleX = (pxPerUnit * 2) / canvasWidthPx
    const scaleY = (pxPerUnit * 2) / canvasHeightPx

    this.viewMatrix.loadIdentity()
    this.viewMatrix.scale(scaleX, scaleY)

    const renderer = this.getRenderer()
    renderer.setMvpMatrix(this.viewMatrix)
    renderer.setRenderState(gl.getParameter(gl.FRAMEBUFFER_BINDING) as WebGLFramebuffer, [0, 0, canvasWidthPx, canvasHeightPx])
    renderer.drawModel()
  }
}
