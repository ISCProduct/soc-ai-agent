'use client'

import { useEffect, useState } from 'react'
import {
  Alert,
  Box,
  Card,
  CardContent,
  CircularProgress,
  FormControlLabel,
  Switch,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { BACKEND_URL } from '@/lib/config'

const ENDPOINT = `${BACKEND_URL}/api/user/training-consent`

/**
 * この画面に出している同意の文言の版（#2168）。保存のたびに送り、サーバーは今の版と一致したときだけ記録する。
 * 下の説明文・箇条書き・スイッチの文言を変えたら、バックエンドの `TrainingConsentTextVersion`
 * （Backend/internal/models/training_consent_event.go）と一緒に上げること（両者が同じことはバックエンドのテストで確かめる）。
 * 古いタブ・古いフロントは古い版を送るので、サーバーが 409 で拒否し、新しい文言への同意として記録されない。
 */
export const TRAINING_CONSENT_TEXT_VERSION = '2026-11-11'

async function fetchConsent(): Promise<boolean> {
  const r = await fetch(ENDPOINT, { headers: authService.getUserFetchHeaders() })
  if (!r.ok) throw new Error('failed')
  const data: { allow_training_use?: boolean } = await r.json()
  return data.allow_training_use === true
}

/**
 * 面接データを採点モデルの学習に使うことへの同意（#2167）。
 *
 * 初期値はオフ（opt-in）。スイッチを切り替えた時点で保存する。希望条件のフォーム（ScoutPreferencesCard）に
 * 混ぜないのは、同意は希望条件と無関係で、「保存」を押し忘れて同意したつもり・取り消したつもりになるのを防ぐため。
 * 保存の応答が失敗したら、今の値を取り直して表示する（応答だけ失われて実際は保存されている場合に、
 * 保存前の値を決め打ちで見せない）。取り直せなければスイッチを出さない（#2180 のレビュー）。
 *
 * 同意の文言（下の説明文・箇条書き・スイッチの文言）を変えたら、`TRAINING_CONSENT_TEXT_VERSION` と
 * バックエンドの `TrainingConsentTextVersion` の版を上げること。同意の履歴には、操作したときの文言の版が
 * 記録される（#2168）。上げ忘れると、変更前の文言に同意した人も新しい文言に同意したように記録される。
 * 文言が更新された後の古い画面からの保存は拒否される（409）ので、再読み込みを促してスイッチを止める。
 */
export function TrainingConsentCard() {
  const [allow, setAllow] = useState(false)
  const [loading, setLoading] = useState(true)
  const [loadFailed, setLoadFailed] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)
  // 表示している文言が古い（サーバーが 409 で拒否した）。再読み込みするまで操作させない。
  const [stale, setStale] = useState(false)

  useEffect(() => {
    fetchConsent()
      .then(setAllow)
      .catch(() => setLoadFailed(true))
      .finally(() => setLoading(false))
  }, [])

  const change = async (next: boolean) => {
    const previous = allow
    setAllow(next)
    setSaving(true)
    setError('')
    setSaved(false)
    try {
      const res = await fetch(ENDPOINT, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', ...authService.getUserFetchHeaders() },
        body: JSON.stringify({
          allow_training_use: next,
          consent_text_version: TRAINING_CONSENT_TEXT_VERSION,
        }),
      })
      if (res.status === 409) {
        // 何も保存されていない。古い文言のまま押し直しても同じなので、再読み込みを促す。
        setAllow(previous)
        setStale(true)
        return
      }
      if (!res.ok) throw new Error('failed')
      const data: { allow_training_use?: boolean } = await res.json()
      setAllow(data.allow_training_use === true)
      setSaved(true)
    } catch {
      // 応答だけ失われて、実際には保存されている場合がある。元の値を決め打ちで表示せず、今の値を取り直す。
      // 取り直せなければ、分からない状態を「オン」「オフ」のどちらにも見せないよう、スイッチを出さない。
      setError('保存できたか確認できませんでした。表示している状態が今の設定です。')
      try {
        setAllow(await fetchConsent())
      } catch {
        setAllow(previous)
        setLoadFailed(true)
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card elevation={0} sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px' }}>
      <CardContent>
        <Typography variant="h6" component="h2" gutterBottom>
          AI 面接の改善への協力
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
          あなたの AI 面接の記録（発話とスコア）を、面接の採点モデルを改善するための学習に使ってよいかの設定です。
        </Typography>
        <Box component="ul" sx={{ m: 0, mb: 2, pl: 2.5, color: 'text.secondary', typography: 'body2' }}>
          <li>氏名・メールアドレス・学校名・企業名などは、学習に使う前に別の記号に置き換えます。</li>
          <li>学習以外の目的には使いません。企業や先生に公開されることもありません。</li>
          <li>いつでもオフにできます。オフにすると、すでに学習用に書き出したあなたのデータも削除します。</li>
        </Box>

        {loading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 1 }}>
            <CircularProgress size={24} aria-label="読み込み中" />
          </Box>
        ) : loadFailed ? (
          <Alert severity="error">設定の取得に失敗しました。ページを再読み込みしてください。</Alert>
        ) : (
          <>
            {stale && (
              <Alert severity="warning" sx={{ mb: 1 }}>
                同意の文言が更新されたため、保存しませんでした。画面を再読み込みして、新しい文言を確認してから操作してください。
              </Alert>
            )}
            {error && (
              <Alert severity="error" sx={{ mb: 1 }}>
                {error}
              </Alert>
            )}
            {saved && (
              <Alert severity="success" sx={{ mb: 1 }}>
                {allow ? '学習への利用に同意しました' : '学習への利用をオフにしました'}
              </Alert>
            )}
            <FormControlLabel
              control={
                <Switch
                  checked={allow}
                  disabled={saving || stale}
                  onChange={(e) => void change(e.target.checked)}
                />
              }
              label="面接の記録を、採点モデルの学習に使うことに同意する"
            />
          </>
        )}
      </CardContent>
    </Card>
  )
}
