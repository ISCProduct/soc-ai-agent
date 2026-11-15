import type { RefObject } from 'react'
import { Box, Typography } from '@mui/material'
import VideocamOffIcon from '@mui/icons-material/VideocamOff'
import { UI } from '@/lib/design/tokens'
import styles from '../interview.module.css'
import DeviceToggles from './DeviceToggles'
import PermissionError from './PermissionError'
import { previewFrameSx } from './lobbyStyles'

interface CameraPreviewProps {
  userName: string
  videoRef: RefObject<HTMLVideoElement | null>
  permissionError: string | null
  cameraEnabled: boolean
  micEnabled: boolean
  onRetryPermissions: () => void
  onToggleCamera: () => void
  onToggleMic: () => void
}

/**
 * 16:9 のカメラプレビュー。
 * 許可エラー中・カメラオフ中も <video> は DOM に残し、ref を親が持つ stream に繋ぎ直せるようにする
 * （表示だけ隠す）。
 */
export default function CameraPreview({
  userName,
  videoRef,
  permissionError,
  cameraEnabled,
  micEnabled,
  onRetryPermissions,
  onToggleCamera,
  onToggleMic,
}: CameraPreviewProps) {
  const hideVideo = Boolean(permissionError) || !cameraEnabled
  return (
    <Box sx={previewFrameSx}>
      <video ref={videoRef} muted playsInline className={`${styles.cameraVideo} ${hideVideo ? styles.cameraVideoHidden : ''}`} />
      {permissionError ? (
        <PermissionError message={permissionError} onRetry={onRetryPermissions} />
      ) : (
        !cameraEnabled && (
          <Box sx={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 1 }}>
            <VideocamOffIcon sx={{ color: UI.nightMuted, fontSize: 48 }} />
            <Typography sx={{ color: UI.nightMuted, fontSize: 13 }}>カメラはオフです</Typography>
          </Box>
        )
      )}
      <Typography sx={{ position: 'absolute', top: 12, left: 12, px: 1.5, py: 0.5, borderRadius: 1, bgcolor: UI.nightPanel, color: UI.nightInk, fontSize: 13 }}>
        {userName || 'あなた'}
      </Typography>
      <DeviceToggles cameraEnabled={cameraEnabled} micEnabled={micEnabled} onToggleCamera={onToggleCamera} onToggleMic={onToggleMic} />
    </Box>
  )
}
