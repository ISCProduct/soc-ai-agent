import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js'
import { VRMLoaderPlugin } from '@pixiv/three-vrm'
import { DRACOLoader } from 'three/examples/jsm/loaders/DRACOLoader.js'
import type { GLTF } from 'three/examples/jsm/loaders/GLTFLoader.js'

const avatarCache = new Map<string, GLTF>()
const loadingPromises = new Map<string, Promise<GLTF>>()

// VRM を先に探す。VRM は humanoid のボーン割り当てと表情が規格化されているので、
// うなずき・まばたき・口の動きを実装依存なしに動かせる（#1603 の調査で、
// 以前入っていた Tripo 製の GLB は skins 0 / morph 0 で静止メッシュだった）。
const AVATAR_PATHS = {
  male: ['/avatars/male-avatar.vrm', '/avatars/male-avatar.glb'],
  female: ['/avatars/female-avatar.vrm', '/avatars/female-avatar.glb'],
} as const

const READY_PLAYER_ME_FALLBACK = {
  // Using Ready Player Me demo avatars - replace with your own custom avatars
  male: 'https://models.readyplayer.me/6746bc1f14c5f70f03c7c45a.glb?morphTargets=Oculus+Visemes&compression=draco',
  female: 'https://models.readyplayer.me/6746bdc914c5f70f03c7c45b.glb?morphTargets=Oculus+Visemes&compression=draco',
} as const

export type AvatarGender = 'male' | 'female'

/**
 * Load a 3D avatar model with caching and fallback support
 * @param gender - The gender of the avatar to load ('male' or 'female')
 * @returns Promise that resolves to the loaded GLTF model
 * @throws Error if loading fails after trying both local and remote sources
 */
export async function loadAvatar(gender: AvatarGender): Promise<GLTF> {
  const cacheKey = `avatar-${gender}`

  // Return cached avatar if available
  if (avatarCache.has(cacheKey)) {
    return avatarCache.get(cacheKey)!
  }

  // Return existing loading promise if in progress
  if (loadingPromises.has(cacheKey)) {
    return loadingPromises.get(cacheKey)!
  }

  // Create new loading promise
  const loadingPromise = loadAvatarInternal(gender)
  loadingPromises.set(cacheKey, loadingPromise)

  try {
    const gltf = await loadingPromise
    avatarCache.set(cacheKey, gltf)
    return gltf
  } finally {
    loadingPromises.delete(cacheKey)
  }
}

/**
 * Internal function to load avatar with fallback logic
 */
async function loadAvatarInternal(gender: AvatarGender): Promise<GLTF> {
  const loader = createGLTFLoader()

  // VRM → GLB の順に試す
  for (const localPath of AVATAR_PATHS[gender]) {
    try {
      const gltf = await loadWithTimeout(loader, localPath, 10000)
      if (!hasMorphTargets(gltf) && !gltf.userData?.vrm) {
        console.warn(
          `[AvatarLoader] ${localPath} は表情（morph target）も VRM の表情も持ちません。` +
          '口の動きは顎ボーンがあればそちらで代替しますが、無ければ動きません。'
        )
      }
      return gltf
    } catch {
      // 次の候補へ
    }
  }
  console.warn(
    `[AvatarLoader] ローカルのアバターが見つかりません（${AVATAR_PATHS[gender].join(' / ')}）。` +
    'Ready Player Me のフォールバックを試します'
  )

  // Fallback to Ready Player Me CDN
  const fallbackUrl = READY_PLAYER_ME_FALLBACK[gender]
  try {
    const gltf = await loadWithTimeout(loader, fallbackUrl, 15000)
    return gltf
  } catch {
    throw new Error(
      `アバターの読み込みに失敗しました（${gender}）。` +
      `frontend/public/avatars/ に ${gender}-avatar.vrm を置いてください（README 参照）。`
    )
  }
}

/**
 * Create a GLTF loader with DRACO compression support
 */
function createGLTFLoader(): GLTFLoader {
  const loader = new GLTFLoader()

  // .vrm は glTF なので GLTFLoader で読める。プラグインを入れると
  // gltf.userData.vrm に humanoid / expressionManager が入る。
  loader.register((parser) => new VRMLoaderPlugin(parser))

  // Set up DRACO loader for compressed models
  const dracoLoader = new DRACOLoader()
  dracoLoader.setDecoderPath('https://www.gstatic.com/draco/versioned/decoders/1.5.6/')
  // preload() は外部CDNへの非同期フェッチを開始するため、ネットワーク不可環境では
  // Failed to fetch エラーが Promise チェーンを通じてアバターロード全体を失敗させる。
  // デコーダーはモデルの初回ロード時に自動的に取得されるため preload() は不要。
  loader.setDRACOLoader(dracoLoader)

  return loader
}

/**
 * Load a GLTF model with timeout
 */
function loadWithTimeout(
  loader: GLTFLoader,
  url: string,
  timeoutMs: number
): Promise<GLTF> {
  return new Promise((resolve, reject) => {
    const timeoutId = setTimeout(() => {
      reject(new Error(`Loading timeout after ${timeoutMs}ms`))
    }, timeoutMs)

    loader.load(
      url,
      (gltf) => {
        clearTimeout(timeoutId)
        resolve(gltf)
      },
      undefined,
      (error) => {
        clearTimeout(timeoutId)
        reject(error)
      }
    )
  })
}

/**
 * Returns true if the loaded GLTF has at least one mesh with morph targets.
 */
function hasMorphTargets(gltf: GLTF): boolean {
  let found = false
  gltf.scene.traverse((child: any) => {
    if (!found && child.isMesh && child.morphTargetDictionary) {
      if (Object.keys(child.morphTargetDictionary).length > 0) found = true
    }
  })
  return found
}

/**
 * Clear the avatar cache
 */
export function clearAvatarCache(): void {
  avatarCache.clear()
}

/**
 * Preload avatars for both genders
 */
export async function preloadAvatars(): Promise<void> {
  await Promise.all([
    loadAvatar('male').catch(e => console.error('Failed to preload male avatar:', e)),
    loadAvatar('female').catch(e => console.error('Failed to preload female avatar:', e)),
  ])
}
