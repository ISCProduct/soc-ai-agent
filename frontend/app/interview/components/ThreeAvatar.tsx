'use client'

import { useEffect, useRef, useState } from 'react'
import * as THREE from 'three'
import { clone as cloneSkeleton } from 'three/examples/jsm/utils/SkeletonUtils.js'
import { Box } from '@mui/material'
import { loadAvatar, type AvatarGender } from '@/lib/avatar-loader'
import { LipsyncManager } from '@/lib/lipsync-manager'

interface ThreeAvatarProps {
  gender: AvatarGender
  audioStream: MediaStream | null
  level: number      // AI audio amplitude 0–1 (drives mouth open shape key)
  speaking: boolean
}

// ─── Shape key discovery ──────────────────────────────────────────────────────
// Patterns tried in order; first match wins.
const MOUTH_OPEN_PATTERNS = [
  // Generic English
  'mouth_open', 'mouthOpen', 'Mouth_Open', 'MouthOpen',
  'mouth.open', 'Mouth.Open', 'open_mouth', 'OpenMouth',
  // Jaw variants
  'jaw_open', 'jawOpen', 'Jaw_Open', 'JawOpen', 'jaw.open',
  // Vowel "a" variants (most common in Japanese VRM/GLB models)
  'mouth_a', 'mouthA', 'Mouth_A', 'mouth_aa', 'mouthAa',
  // Japanese (Blender export preserves Japanese names)
  'あ', '口_あ', 'mouth_あ',
  // Misc
  'A', 'a',
]

interface MouthTarget {
  mesh: THREE.Mesh
  index: number
  smoothed: number  // current smoothed weight
}

/** Traverse the loaded GLTF scene and find the best mouth-open morph target. */
function findMouthTarget(root: THREE.Object3D): MouthTarget | null {
  // First pass: collect all morph target info and log it
  const allKeys: { meshName: string; keys: string[] }[] = []
  root.traverse((child) => {
    if (child instanceof THREE.Mesh && child.morphTargetDictionary) {
      const keys = Object.keys(child.morphTargetDictionary)
      if (keys.length > 0) {
        allKeys.push({ meshName: child.name || '(unnamed)', keys })
      }
    }
  })

  if (allKeys.length > 0) {
    console.log('[ThreeAvatar] All morph targets found in model:')
    allKeys.forEach(({ meshName, keys }) =>
      console.log(`  mesh "${meshName}":`, keys)
    )
  } else {
    console.log('[ThreeAvatar] No morph targets found in model – lipsync will use head animation only')
    return null
  }

  // Second pass: exact-match against known patterns
  for (const pattern of MOUTH_OPEN_PATTERNS) {
    let found: MouthTarget | null = null
    root.traverse((child) => {
      if (found) return
      if (child instanceof THREE.Mesh && child.morphTargetDictionary) {
        const idx = child.morphTargetDictionary[pattern]
        if (idx !== undefined) {
          found = { mesh: child, index: idx, smoothed: 0 }
          console.log(`[ThreeAvatar] Using mouth shape key: "${pattern}" (index ${idx}) on mesh "${child.name}"`)
        }
      }
    })
    if (found) return found
  }

  // Third pass: case-insensitive partial match on 'mouth' / 'jaw' / 'open' / 'あ'
  let partial: MouthTarget | null = null
  root.traverse((child) => {
    if (partial) return
    if (child instanceof THREE.Mesh && child.morphTargetDictionary) {
      for (const [key, idx] of Object.entries(child.morphTargetDictionary)) {
        const lower = key.toLowerCase()
        if (lower.includes('mouth') || lower.includes('jaw') || lower.includes('open') || key === 'あ') {
          partial = { mesh: child, index: idx, smoothed: 0 }
          console.log(`[ThreeAvatar] Partial-match mouth shape key: "${key}" (index ${idx}) on mesh "${child.name}"`)
          break
        }
      }
    }
  })

  if (!partial) {
    console.log('[ThreeAvatar] No mouth shape key matched – lipsync drives head animation only')
  }
  return partial
}

// ─────────────────────────────────────────────────────────────────────────────

export default function ThreeAvatar({ gender, audioStream, level, speaking }: ThreeAvatarProps) {
  const [useFallback, setUseFallback] = useState(false)
  const [isLoading, setIsLoading]   = useState(true)
  const containerRef    = useRef<HTMLDivElement | null>(null)
  const levelRef        = useRef(level)
  const speakingRef     = useRef(speaking)
  const lipsyncMgrRef   = useRef<LipsyncManager | null>(null)
  const avatarMeshRef   = useRef<THREE.Object3D | null>(null)
  const mouthTargetRef  = useRef<MouthTarget | null>(null)
  // Jaw bone for models without morph targets
  const jawBoneRef      = useRef<THREE.Bone | null>(null)
  const jawRestRotRef   = useRef<THREE.Euler | null>(null)
  const jawSmoothedRef  = useRef(0)
  // Morph meshes for Oculus viseme fallback (RPM-style models)
  const visemeMeshesRef = useRef<THREE.Mesh[]>([])

  useEffect(() => { levelRef.current = level },    [level])
  useEffect(() => { speakingRef.current = speaking }, [speaking])

  useEffect(() => {
    if (lipsyncMgrRef.current && audioStream) {
      lipsyncMgrRef.current.updateAudioStream(audioStream)
    }
  }, [audioStream])

  useEffect(() => {
    if (!containerRef.current || useFallback) return

    const container = containerRef.current
    const width  = container.clientWidth
    const height = container.clientHeight

    // ── Three.js scene ──────────────────────────────────────────────────────
    const scene    = new THREE.Scene()
    const camera   = new THREE.PerspectiveCamera(40, width / height, 0.1, 20)
    camera.position.set(0, 0.8, 2.8)

    const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true })
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
    renderer.setSize(width, height)
    renderer.outputColorSpace = THREE.SRGBColorSpace
    renderer.shadowMap.enabled = true
    renderer.shadowMap.type = THREE.PCFSoftShadowMap
    container.appendChild(renderer.domElement)

    // Lighting
    scene.add(new THREE.AmbientLight(0xffffff, 1.2))
    const key = new THREE.DirectionalLight(0xffffff, 1.5)
    key.position.set(2, 2, 3); key.castShadow = true; scene.add(key)
    const fill = new THREE.DirectionalLight(0xffffff, 0.6)
    fill.position.set(-2, 1, -2); scene.add(fill)
    const rim = new THREE.DirectionalLight(0xffffff, 0.4)
    rim.position.set(0, 2, -3); scene.add(rim)

    const avatarGroup = new THREE.Group()
    scene.add(avatarGroup)

    const halo = new THREE.Mesh(
      new THREE.CircleGeometry(1.5, 40),
      new THREE.MeshBasicMaterial({ color: 0xf7f4ec, transparent: true, opacity: 0.3 })
    )
    halo.position.set(0, 0.5, -0.5)
    scene.add(halo)

    let disposed = false
    let frameId  = 0

    // ── Animation loop ──────────────────────────────────────────────────────
    const MOUTH_SMOOTHING_ATTACK  = 0.22
    const MOUTH_SMOOTHING_RELEASE = 0.14

    const animate = () => {
      if (disposed) return
      const t    = performance.now() / 1000
      const talk = Math.min(1, levelRef.current * 1.2)

      // Subtle head bob / tilt
      avatarGroup.rotation.y = Math.sin(t * 0.5) * 0.04 + (talk - 0.2) * 0.02
      avatarGroup.rotation.x = Math.sin(t * 0.7) * 0.02
      avatarGroup.position.y = Math.sin(t * 1.0) * 0.03

      // Breathing / speaking pulse on scale
      if (speakingRef.current) {
        const pulse = 1 + talk * 0.015
        avatarGroup.scale.set(pulse, pulse, pulse)
      } else {
        const breathe = 1 + Math.sin(t * 0.8) * 0.005
        avatarGroup.scale.set(breathe, breathe, breathe)
      }

      // ── Mouth shape key (primary path) ─────────────────────────────────
      const mt = mouthTargetRef.current
      if (mt && mt.mesh.morphTargetInfluences) {
        const target = Math.min(1, levelRef.current * 1.4)
        if (target > mt.smoothed) {
          mt.smoothed += (target - mt.smoothed) * MOUTH_SMOOTHING_ATTACK
        } else {
          mt.smoothed += (target - mt.smoothed) * MOUTH_SMOOTHING_RELEASE
        }
        mt.mesh.morphTargetInfluences[mt.index] = mt.smoothed
      }

      // ── Jaw bone rotation (for models without morph targets) ───────────
      const jaw = jawBoneRef.current
      if (jaw && jawRestRotRef.current) {
        const target = Math.min(1, levelRef.current * 1.4)
        const prev = jawSmoothedRef.current
        jawSmoothedRef.current = prev + (target - prev) * (target > prev ? MOUTH_SMOOTHING_ATTACK : MOUTH_SMOOTHING_RELEASE)
        // Rotate jaw downward (positive X = mouth opens for typical rig)
        jaw.rotation.x = jawRestRotRef.current.x + jawSmoothedRef.current * 0.35
      }

      // ── Oculus viseme fallback (for RPM-style models with no mouth_open) ─
      if (!mt && !jaw && lipsyncMgrRef.current && visemeMeshesRef.current.length > 0) {
        const visemes = lipsyncMgrRef.current.getCurrentVisemes()
        visemeMeshesRef.current.forEach((mesh) => {
          if (!mesh.morphTargetDictionary || !mesh.morphTargetInfluences) return
          Object.entries(visemes).forEach(([name, weight]) => {
            const idx = mesh.morphTargetDictionary![name]
            if (idx !== undefined) {
              const cur = mesh.morphTargetInfluences![idx] ?? 0
              mesh.morphTargetInfluences![idx] = cur + (weight - cur) * 0.2
            }
          })
        })
      }

      renderer.render(scene, camera)
      frameId = requestAnimationFrame(animate)
    }

    // ── Load avatar ─────────────────────────────────────────────────────────
    const loadModel = async () => {
      try {
        setIsLoading(true)
        console.log(`[ThreeAvatar] Loading ${gender} avatar…`)

        const gltf  = await loadAvatar(gender)
        if (disposed) return

        // キャッシュされた gltf.scene を直接使うとクリーンアップ時の dispose が
        // キャッシュ上のテクスチャを破棄し、次回マウント時に blob URL エラーが発生する。
        // SkeletonUtils.clone でスケルトンを含むディープクローンを作り、
        // マテリアルも個別にクローンして各インスタンスが独立した状態を持つようにする。
        const model = cloneSkeleton(gltf.scene) as THREE.Group
        model.traverse((child) => {
          if ((child as THREE.Mesh).isMesh) {
            const mesh = child as THREE.Mesh
            if (Array.isArray(mesh.material)) {
              mesh.material = mesh.material.map((m) => m.clone())
            } else {
              mesh.material = (mesh.material as THREE.Material).clone()
            }
          }
        })

        // ── Auto-normalize model size and position ────────────────────────
        // RPM / standard glTF: front faces +Z (no rotation needed)
        // Tripo3D static exports: front faces +X (needs -90° Y rotation)
        // Detect by checking if model has a skeleton (RPM) or is a bare mesh (Tripo)
        const hasSkeleton = (() => { let s = false; model.traverse(c => { if ((c as any).isBone) s = true }); return s })()
        model.rotation.set(0, hasSkeleton ? 0 : -Math.PI / 2, 0)
        model.scale.set(1, 1, 1)
        model.position.set(0, 0, 0)
        model.updateMatrixWorld(true)

        const box    = new THREE.Box3().setFromObject(model)
        const size   = box.getSize(new THREE.Vector3())
        const center = box.getCenter(new THREE.Vector3())

        // Scale so the model is TARGET_HEIGHT units tall
        const TARGET_HEIGHT = 1.8
        const s = size.y > 0 ? TARGET_HEIGHT / size.y : 1
        model.scale.setScalar(s)

        // Center horizontally; shift up so head/torso fill the camera view
        // Y offset: move model so its upper-body is in frame (camera looks at Y≈0.8)
        model.position.x = -center.x * s
        model.position.y = -center.y * s + 0.3   // shift up so head/torso fill camera frame
        model.position.z = -center.z * s

        console.log(`[ThreeAvatar] model size=${JSON.stringify(size.toArray().map(v=>+v.toFixed(3)))} scale=${s.toFixed(3)}`)
        // ─────────────────────────────────────────────────────────────────

        avatarGroup.add(model)
        avatarMeshRef.current = model

        // Discover mouth shape key
        mouthTargetRef.current = findMouthTarget(model)

        // ── Jaw bone detection (for models without morph targets) ─────────
        jawBoneRef.current = null
        jawRestRotRef.current = null
        jawSmoothedRef.current = 0
        const JAW_BONE_PATTERNS = [
          'jaw', 'Jaw', 'JAW',
          'mixamorigJaw', 'mixamorig:Jaw',
          'jaw_master', 'lowerjaw', 'LowerJaw', 'lower_jaw',
          'mouth', 'Mouth',
        ]
        // Collect ALL named nodes (Bone or Object3D) for logging + search
        const allNodes: THREE.Object3D[] = []
        model.traverse((child) => { if (child.name) allNodes.push(child) })
        console.log('[ThreeAvatar] All nodes:', allNodes.map(n => `${n.type}:${n.name}`))

        let foundNode: THREE.Object3D | null = null
        // Exact match
        for (const pat of JAW_BONE_PATTERNS) {
          foundNode = allNodes.find(n => n.name === pat) ?? null
          if (foundNode) break
        }
        // Partial match
        if (!foundNode) {
          foundNode = allNodes.find(n =>
            n.name.toLowerCase().includes('jaw') || n.name.toLowerCase().includes('mouth')
          ) ?? null
        }
        if (foundNode) {
          jawBoneRef.current = foundNode as THREE.Bone
          jawRestRotRef.current = foundNode.rotation.clone()
          console.log(`[ThreeAvatar] Jaw node found: "${foundNode.name}" (${foundNode.type})`)
        } else {
          console.log('[ThreeAvatar] No jaw node matched. Node list above — please report bone names.')
        }

        // Collect Oculus-viseme meshes as fallback
        visemeMeshesRef.current = []
        model.traverse((child) => {
          if (child instanceof THREE.Mesh && child.morphTargetDictionary) {
            const keys = Object.keys(child.morphTargetDictionary)
            if (keys.some(k => k.startsWith('viseme_'))) {
              visemeMeshesRef.current.push(child)
            }
          }
        })

        // Init lipsync manager (for Oculus viseme fallback)
        if (audioStream) {
          lipsyncMgrRef.current = new LipsyncManager(audioStream)
        }

        setIsLoading(false)
        animate()
        console.log(`[ThreeAvatar] ${gender} avatar ready`)
      } catch (err) {
        console.error('[ThreeAvatar] Failed to load avatar:', err)
        if (!disposed) setUseFallback(true)
      }
    }

    loadModel()

    const onResize = () => {
      const w = container.clientWidth
      const h = container.clientHeight
      camera.aspect = w / h
      camera.updateProjectionMatrix()
      renderer.setSize(w, h)
    }
    window.addEventListener('resize', onResize)

    return () => {
      disposed = true
      cancelAnimationFrame(frameId)
      window.removeEventListener('resize', onResize)

      lipsyncMgrRef.current?.dispose()
      lipsyncMgrRef.current = null

      renderer.dispose()
      if (renderer.domElement.parentElement === container) {
        container.removeChild(renderer.domElement)
      }

      scene.traverse((obj) => {
        if (obj instanceof THREE.Mesh) {
          obj.geometry.dispose()
          const m = obj.material
          if (Array.isArray(m)) m.forEach(mm => mm.dispose())
          else m.dispose()
        }
      })

      visemeMeshesRef.current = []
      avatarMeshRef.current   = null
      mouthTargetRef.current  = null
      jawBoneRef.current      = null
      jawRestRotRef.current   = null
      console.log('[ThreeAvatar] Cleanup complete')
    }
  }, [gender, useFallback]) // eslint-disable-line react-hooks/exhaustive-deps

  if (useFallback) {
    return <InterviewerFallbackAvatar gender={gender} level={level} speaking={speaking} />
  }

  return (
    <Box
      ref={containerRef}
      sx={{
        width:  { xs: 214, md: 330, lg: 380 },
        height: { xs: 214, md: 330, lg: 380 },
        position: 'relative',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      {isLoading && (
        <Box sx={{
          position: 'absolute', top: '50%', left: '50%',
          transform: 'translate(-50%, -50%)',
          color: 'text.secondary', fontSize: '0.875rem',
        }}>
          アバター読込中...
        </Box>
      )}
    </Box>
  )
}

// ─── SVG fallback ─────────────────────────────────────────────────────────────
function InterviewerFallbackAvatar({
  gender, level, speaking,
}: { gender: 'male' | 'female'; level: number; speaking: boolean }) {
  const mouthOpen  = Math.max(4, Math.min(18, Math.round(4 + level * 20)))
  const hairColor  = gender === 'female' ? '#4f3326' : '#2b2b34'
  const suitColor  = gender === 'female' ? '#48607f' : '#2f4a66'
  const accentColor = gender === 'female' ? '#e6d8c4' : '#c8d6e5'

  return (
    <Box sx={{
      width: { xs: 190, md: 280 }, height: { xs: 190, md: 280 },
      borderRadius: '50%', overflow: 'hidden',
      position: 'relative', display: 'grid', placeItems: 'center',
      background: 'radial-gradient(circle at 48% 30%, #fefcf7 0%, #f2eadf 38%, #d7c7b0 100%)',
      boxShadow: speaking
        ? '0 0 0 10px rgba(30,64,175,0.15), inset 0 -8px 20px rgba(0,0,0,0.1)'
        : 'inset 0 -8px 20px rgba(0,0,0,0.08)',
      transform: speaking ? 'scale(1.01)' : 'scale(1)',
      transition: 'all 0.16s ease',
    }}>
      <svg viewBox="0 0 120 140" width="100%" height="100%"
           style={{ position: 'absolute', top: 0, left: 0 }}>
        <defs>
          <radialGradient id="face-grad" cx="50%" cy="40%">
            <stop offset="0%" stopColor="#ffe8d4" />
            <stop offset="100%" stopColor="#f5d5bd" />
          </radialGradient>
        </defs>
        <ellipse cx="60" cy="52" rx="28" ry="32" fill="url(#face-grad)" />
        <ellipse cx="60" cy="25" rx="30" ry="28" fill={hairColor} />
        <rect x="30" y="28" width="60" height="16" fill={hairColor} />
        <circle cx="50" cy="48" r="2.5" fill="#2c2420" />
        <circle cx="70" cy="48" r="2.5" fill="#2c2420" />
        <ellipse cx="48" cy="52" rx="3" ry="2" fill="#ffb8a0" opacity="0.45" />
        <ellipse cx="72" cy="52" rx="3" ry="2" fill="#ffb8a0" opacity="0.45" />
        <path
          d={`M 52 60 Q 60 ${60 + mouthOpen} 68 60`}
          fill="none" stroke="#c97a6a" strokeWidth="1.8" strokeLinecap="round"
        />
        {speaking && mouthOpen > 8 && (
          <ellipse cx="60" cy={64 + mouthOpen / 2} rx="6" ry={mouthOpen / 2}
                   fill="#4a2020" opacity="0.4" />
        )}
        <rect x="32" y="76" width="56" height="50" rx="2" fill={suitColor} />
        <path d="M 60 76 L 52 86 L 60 95 L 68 86 Z" fill={accentColor} />
        <rect x="32" y="76" width="14" height="40" fill={suitColor} opacity="0.85" />
        <rect x="74" y="76" width="14" height="40" fill={suitColor} opacity="0.85" />
      </svg>
    </Box>
  )
}
