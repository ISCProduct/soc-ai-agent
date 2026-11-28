'use client'

import { useEffect, useState, type ReactNode } from 'react'
import { Box, Button, NativeSelect, Stack } from '@mui/material'
import MicIcon from '@mui/icons-material/Mic'
import VolumeUpIcon from '@mui/icons-material/VolumeUp'
import VideocamIcon from '@mui/icons-material/Videocam'
import { UI } from '@/lib/design/tokens'
import { measureLevel } from '@/lib/interview/ai-audio-meter'
import { canSelectAudioOutput, playTestTone } from '@/lib/interview/audio-output'
import { useMediaDevices } from '../../hooks/useMediaDevices'
import { cardSx, textLinkSx } from './lobbyStyles'

/** 音量メーターのバーの本数。 */
const METER_BARS = 20

/**
 * マイクの音量メーター。毎フレーム測るが、バーの本数が変わったときだけ再描画する。
 * 状態は色だけで伝えず、点いているバーの本数で示す。
 */
function MicMeter({ stream }: { stream: MediaStream | null }) {
  const [lit, setLit] = useState(0)

  useEffect(() => {
    if (!stream || stream.getAudioTracks().length === 0 || typeof AudioContext === 'undefined') return
    const ctx = new AudioContext()
    void ctx.resume().catch(() => {})
    const analyser = ctx.createAnalyser()
    analyser.fftSize = 512
    ctx.createMediaStreamSource(stream).connect(analyser)
    const buf = new Uint8Array(analyser.fftSize)
    let raf = 0
    const tick = () => {
      setLit(Math.round(measureLevel(analyser, buf) * METER_BARS))
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => {
      cancelAnimationFrame(raf)
      void ctx.close().catch(() => {})
      setLit(0)
    }
  }, [stream])

  return (
    <Stack direction="row" spacing={0.75} alignItems="center" role="img" aria-label={`マイクの音量 ${lit}/${METER_BARS}`}>
      <MicIcon sx={{ fontSize: 18, color: UI.inkFaint }} aria-hidden />
      {Array.from({ length: METER_BARS }, (_, i) => (
        <Box key={i} sx={{ width: 5, height: 16, borderRadius: 9999, bgcolor: i < lit ? UI.mark : UI.ruleSoft }} />
      ))}
    </Stack>
  )
}

function DeviceRow({ icon, label, value, devices, onChange }: {
  icon: ReactNode
  label: string
  value: string
  devices: MediaDeviceInfo[]
  onChange: (id: string) => void
}) {
  // 未選択（既定）のときは一覧の先頭を見せる。Chrome は先頭が「既定」の項目になる。
  const shown = devices.some((d) => d.deviceId === value) ? value : (devices[0]?.deviceId ?? '')
  return (
    <Stack direction="row" spacing={1.5} alignItems="center">
      <Box sx={{ color: UI.inkSoft, display: 'flex' }} aria-hidden>{icon}</Box>
      <NativeSelect
        value={shown}
        onChange={(e) => onChange(e.target.value)}
        disabled={devices.length === 0}
        inputProps={{ 'aria-label': label }}
        sx={{ flex: 1, minWidth: 0, bgcolor: UI.field, px: 1.5, py: 0.5, border: `1px solid ${UI.rule}`, borderRadius: 1, '&::before, &::after': { display: 'none' } }}
      >
        {devices.length === 0 && <option value="">{label}が見つかりません</option>}
        {devices.map((d, i) => (
          <option key={d.deviceId || i} value={d.deviceId}>{d.label || `${label} ${i + 1}`}</option>
        ))}
      </NativeSelect>
    </Stack>
  )
}

interface DeviceSettingsProps {
  /** 準備画面のプレビューに繋いだストリーム。取れるまで（許可前）は一覧を読まない */
  stream: MediaStream | null
  micId: string
  cameraId: string
  speakerId: string
  onSelectMic: (id: string) => void
  onSelectCamera: (id: string) => void
  onSelectSpeaker: (id: string) => void
}

/** プレビューの下に置く、マイク・スピーカー・カメラの選択（#2634）。 */
export default function DeviceSettings({ stream, micId, cameraId, speakerId, onSelectMic, onSelectCamera, onSelectSpeaker }: DeviceSettingsProps) {
  const { mics, speakers, cameras } = useMediaDevices(stream !== null)
  // スピーカーを選べないブラウザ（AudioContext.setSinkId が無い）では欄を出さず、既定のスピーカーで鳴らす
  const [speakerSelectable] = useState(canSelectAudioOutput)

  // 選んでいた機器が抜かれたら既定に戻す
  useEffect(() => {
    if (micId && mics.length > 0 && !mics.some((d) => d.deviceId === micId)) onSelectMic('')
  }, [micId, mics, onSelectMic])
  useEffect(() => {
    if (cameraId && cameras.length > 0 && !cameras.some((d) => d.deviceId === cameraId)) onSelectCamera('')
  }, [cameraId, cameras, onSelectCamera])
  useEffect(() => {
    if (speakerId && speakers.length > 0 && !speakers.some((d) => d.deviceId === speakerId)) onSelectSpeaker('')
  }, [speakerId, speakers, onSelectSpeaker])

  return (
    <Stack spacing={1.5} sx={{ ...cardSx, mt: 2 }}>
      <DeviceRow icon={<MicIcon />} label="マイク" value={micId} devices={mics} onChange={onSelectMic} />
      <Box sx={{ pl: 4.5 }}>
        <MicMeter stream={stream} />
      </Box>
      {speakerSelectable && (
        <DeviceRow icon={<VolumeUpIcon />} label="スピーカー" value={speakerId} devices={speakers} onChange={onSelectSpeaker} />
      )}
      <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
        <Button onClick={() => { void playTestTone().catch(() => {}) }} sx={textLinkSx}>
          スピーカーをテスト
        </Button>
      </Box>
      <DeviceRow icon={<VideocamIcon />} label="カメラ" value={cameraId} devices={cameras} onChange={onSelectCamera} />
    </Stack>
  )
}
