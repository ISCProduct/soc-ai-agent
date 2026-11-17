import { Box, Typography } from '@mui/material'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import CloudUploadOutlinedIcon from '@mui/icons-material/CloudUploadOutlined'
import { UI } from '@/lib/design/tokens'

export type VideoUploadStatus = 'idle' | 'uploading' | 'done' | 'error'

/** 動画アップロードの1行。完了・失敗はアイコンと文言で示し、色だけに頼らない */
export default function UploadRow({ status, progress }: { status: VideoUploadStatus; progress: number }) {
  if (status === 'idle') return null
  const label =
    status === 'error' ? '動画のアップロードに失敗しました。ネットワーク接続を確認してください' : '動画のアップロード'
  const value = status === 'uploading' ? `${progress}%` : status === 'done' ? '100%' : null
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
      {status === 'done' && <CheckCircleIcon fontSize="small" sx={{ color: 'success.main' }} />}
      {status === 'uploading' && <CloudUploadOutlinedIcon fontSize="small" sx={{ color: UI.mark }} />}
      {status === 'error' && <ErrorOutlineIcon fontSize="small" sx={{ color: UI.flag }} />}
      <Typography variant="body2" sx={{ flex: 1, color: status === 'error' ? UI.flag : UI.ink }}>{label}</Typography>
      {value && <Typography variant="body2" sx={{ color: UI.inkSoft }}>{value}</Typography>}
    </Box>
  )
}
