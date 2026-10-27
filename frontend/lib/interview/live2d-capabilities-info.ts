/**
 * `inspectLive2DModel`（live2d-params.ts）に渡す形を組み立てる、純粋なデータ変換。
 *
 * `live2d-framework-model.ts`に置かず独立ファイルにしてあるのは、あちらがvendorの
 * Cubism Web Framework を import していて、Framework側の一部ファイル（例:
 * `model/cubismmodel.ts`）が enum の初期化式で `Live2DCubismCore`（Core読み込み後に
 * 生える前提のグローバル）をモジュール評価時に参照しており、Core無しでimportすると
 * `ReferenceError: Live2DCubismCore is not defined` になる（Jestのjsdomや、Core読込前の
 * SSR環境で起きる）。このファイルはFrameworkに一切依存しないので、どこからでも安全にimportできる。
 */

export interface Live2DFrameworkCapabilitiesInfo {
  parameterIds: readonly string[]
  lipSyncIds?: readonly string[]
  eyeBlinkIds?: readonly string[]
}

export interface IdGroupAccessor {
  count: number
  idAt: (i: number) => string
}

export function buildCapabilitiesInfo(
  parameterIds: readonly string[],
  lipSync: IdGroupAccessor | null,
  eyeBlink: IdGroupAccessor | null,
): Live2DFrameworkCapabilitiesInfo {
  const listOf = (g: IdGroupAccessor | null): string[] | undefined => {
    if (!g) return undefined
    const ids: string[] = []
    for (let i = 0; i < g.count; i++) ids.push(g.idAt(i))
    return ids
  }
  return {
    parameterIds,
    lipSyncIds: listOf(lipSync),
    eyeBlinkIds: listOf(eyeBlink),
  }
}
