import type { RefCallback } from 'react'
import { Box } from '@mui/material'
import VideocamOffIcon from '@mui/icons-material/VideocamOff'

import { CALL } from '../../callTokens'

interface Props {
  userName: string
  cameraEnabled: boolean
  /** video 要素がマウントした瞬間にストリームをアタッチするための callback ref（page-content 側で理由を解説） */
  videoRef: RefCallback<HTMLVideoElement>
}

/** 面接官タイル右上の自分の小窓。カメラ off でも video は外さず隠す（ref を保つため）。 */
export default function SelfView({ userName, cameraEnabled, videoRef }: Props) {
  return (
    <Box
      role="group"
      aria-label={`あなたの映像${userName ? `（${userName}）` : ''}`}
      sx={{
        position: 'absolute', top: 12, right: 12, zIndex: 1,
        width: 'clamp(96px, 22%, 176px)', aspectRatio: '16 / 10',
        borderRadius: 1.5, overflow: 'hidden', bgcolor: CALL.control,
      }}
    >
      <Box
        component="video"
        ref={videoRef}
        muted
        playsInline
        // 自分の映像は鏡像で表示する
        sx={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover', transform: 'scaleX(-1)', display: cameraEnabled ? 'block' : 'none' }}
      />
      {!cameraEnabled && (
        <Box sx={{ position: 'absolute', inset: 0, display: 'grid', placeItems: 'center', color: CALL.muted }}>
          <VideocamOffIcon aria-label="カメラはオフです" />
        </Box>
      )}
    </Box>
  )
}
