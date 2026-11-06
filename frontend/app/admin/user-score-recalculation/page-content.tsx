'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import {
  Alert,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  LinearProgress,
  Paper,
  Stack,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { useRequirePlatformAdmin } from '@/lib/admin/use-require-platform-admin'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'

type Preview = {
  calculation_version: string
  eligible_sessions: number
  unsupported_sessions: number
  estimated_llm_calls: number
  estimated_cost_usd: number
  cost_estimate_note: string
  active_job: Job | null
}

type JobStatus = 'running' | 'paused' | 'completed' | 'cancelled'

type Job = {
  id: number
  calculation_version: string
  status: JobStatus
  total_sessions: number
  processed_sessions: number
  failed_sessions: number
  estimated_cost_usd: number
  last_error?: string
}

const JOB_STATUS_LABELS: Record<JobStatus, string> = {
  running: '実行中',
  paused: '一時停止中',
  completed: '完了',
  cancelled: '中止',
}

const FAILURE_LABELS: Record<string, string> = {
  load_failed: '会話履歴を読み込めませんでした',
  no_scores: '採点できる回答がありませんでした',
  save_failed: 'スコアを保存できませんでした',
  target_changed: '途中で履歴書・ES・面接のスコアが反映されたので、再計算しませんでした',
  matching_failed: 'スコアは保存しましたが、企業マッチングを更新できませんでした',
}

function isJobStatus(value: unknown): value is JobStatus {
  return value === 'running' || value === 'paused' || value === 'completed' || value === 'cancelled'
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function parsePreview(value: unknown): Preview | null {
  if (!isRecord(value)) return null
  const { calculation_version, eligible_sessions, unsupported_sessions, estimated_llm_calls, estimated_cost_usd, cost_estimate_note } = value
  if (
    typeof calculation_version !== 'string' ||
    typeof eligible_sessions !== 'number' ||
    typeof unsupported_sessions !== 'number' ||
    typeof estimated_llm_calls !== 'number' ||
    typeof estimated_cost_usd !== 'number' ||
    typeof cost_estimate_note !== 'string'
  ) return null
  const active_job = parseJob(value.active_job)
  if (value.active_job !== null && !active_job) return null
  return { calculation_version, eligible_sessions, unsupported_sessions, estimated_llm_calls, estimated_cost_usd, cost_estimate_note, active_job }
}

function parseJob(value: unknown): Job | null {
  if (!isRecord(value)) return null
  const { id, calculation_version, status, total_sessions, processed_sessions, failed_sessions, estimated_cost_usd, last_error } = value
  if (
    typeof id !== 'number' ||
    typeof calculation_version !== 'string' ||
    !isJobStatus(status) ||
    typeof total_sessions !== 'number' ||
    typeof processed_sessions !== 'number' ||
    typeof failed_sessions !== 'number' ||
    typeof estimated_cost_usd !== 'number'
  ) return null
  return {
    id,
    calculation_version,
    status,
    total_sessions,
    processed_sessions,
    failed_sessions,
    estimated_cost_usd,
    last_error: typeof last_error === 'string' ? last_error : undefined,
  }
}

function errorMessage(value: unknown, fallback: string): string {
  if (isRecord(value) && typeof value.message === 'string') return value.message
  if (isRecord(value) && typeof value.error === 'string') return value.error
  return fallback
}

export default function PageContent() {
  const platformReady = useRequirePlatformAdmin()
  const [preview, setPreview] = useState<Preview | null>(null)
  const [job, setJob] = useState<Job | null>(null)
  const [loading, setLoading] = useState(true)
  const [processing, setProcessing] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [cancelConfirmOpen, setCancelConfirmOpen] = useState(false)
  const [rollbackConfirmOpen, setRollbackConfirmOpen] = useState(false)
  const [rollbackMessage, setRollbackMessage] = useState('')
  const [dryRunText, setDryRunText] = useState('')
  const [dryRunning, setDryRunning] = useState(false)
  const [error, setError] = useState('')
  const stopRequested = useRef(false)
  const activeJobID = useRef<number | null>(null)

  const request = useCallback(async (path: string, method: 'GET' | 'POST' = 'GET'): Promise<unknown> => {
    const response = await fetch(`/api/admin/user-score-recalculation/${path}`, {
      method,
      headers: authService.getAdminFetchHeaders(),
    })
    const body: unknown = await response.json()
    if (!response.ok) throw new Error(errorMessage(body, `HTTP ${response.status}`))
    return body
  }, [])

  const runBatches = useCallback(async (jobID: number) => {
    if (activeJobID.current === jobID) return
    activeJobID.current = jobID
    stopRequested.current = false
    setProcessing(true)
    setError('')
    try {
      while (!stopRequested.current) {
        const nextJob = parseJob(await request(`jobs/${jobID}/run`, 'POST'))
        if (!nextJob) throw new Error('再計算の進捗を読み取れませんでした')
        setJob(nextJob)
        if (nextJob.status !== 'running') break
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : '再計算に失敗しました')
    } finally {
      activeJobID.current = null
      setProcessing(false)
    }
  }, [request])

  const loadData = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const newPreview = parsePreview(await request('preview'))
      if (!newPreview) throw new Error('再計算対象の見積もりを読み取れませんでした')
      setPreview(newPreview)
      const storedID = window.localStorage.getItem('user-score-recalculation-job')
      if (newPreview.active_job) {
        // 自分が始めたジョブだけ自動で続ける。他の管理者のジョブは、開いただけで
        // 費用が発生しないように「続行」を押すまで動かさない。
        setJob(newPreview.active_job)
        if (newPreview.active_job.status === 'running' && storedID === String(newPreview.active_job.id)) {
          void runBatches(newPreview.active_job.id)
        }
      } else if (storedID) {
        // 進行中のジョブが無ければ、前回のジョブの結果を表示する。
        const storedJob = parseJob(await request(`jobs/${encodeURIComponent(storedID)}`))
        if (storedJob) setJob(storedJob)
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : '再計算対象を取得できませんでした')
    } finally {
      setLoading(false)
    }
  }, [request, runBatches])

  useEffect(() => {
    void loadData()
    return () => { stopRequested.current = true }
  }, [loadData])

  const startJob = async () => {
    setConfirmOpen(false)
    setError('')
    try {
      const newJob = parseJob(await request('jobs', 'POST'))
      if (!newJob) throw new Error('再計算ジョブを読み取れませんでした')
      setJob(newJob)
      window.localStorage.setItem('user-score-recalculation-job', String(newJob.id))
      if (newJob.status === 'running') void runBatches(newJob.id)
    } catch (e) {
      setError(e instanceof Error ? e.message : '再計算ジョブを開始できませんでした')
    }
  }

  const pauseJob = async () => {
    if (!job) return
    stopRequested.current = true
    try {
      const updated = parseJob(await request(`jobs/${job.id}/pause`, 'POST'))
      if (!updated) throw new Error('一時停止状態を読み取れませんでした')
      setJob(updated)
    } catch (e) {
      setError(e instanceof Error ? e.message : '再計算を一時停止できませんでした')
    }
  }

  const resumeJob = async () => {
    if (!job) return
    try {
      const updated = parseJob(await request(`jobs/${job.id}/resume`, 'POST'))
      if (!updated) throw new Error('再開状態を読み取れませんでした')
      setJob(updated)
      void runBatches(updated.id)
    } catch (e) {
      setError(e instanceof Error ? e.message : '再計算を再開できませんでした')
    }
  }

  const continueJob = () => {
    if (!job) return
    window.localStorage.setItem('user-score-recalculation-job', String(job.id))
    void runBatches(job.id)
  }

  const cancelJob = async () => {
    setCancelConfirmOpen(false)
    if (!job) return
    setError('')
    try {
      const updated = parseJob(await request(`jobs/${job.id}/cancel`, 'POST'))
      if (!updated) throw new Error('中止状態を読み取れませんでした')
      setJob(updated)
    } catch (e) {
      setError(e instanceof Error ? e.message : '再計算を中止できませんでした')
    }
  }

  const runDryRun = async () => {
    setError('')
    setDryRunning(true)
    try {
      const body = await request('dry-run', 'POST')
      if (!isRecord(body) || typeof body.text !== 'string') throw new Error('ドライランの結果を読み取れませんでした')
      setDryRunText(body.text)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'ドライランに失敗しました')
    } finally {
      setDryRunning(false)
    }
  }

  const rollbackJob = async () => {
    setRollbackConfirmOpen(false)
    if (!job) return
    setError('')
    setRollbackMessage('')
    setProcessing(true)
    let restored = 0
    let skipped = 0
    let matchingFailed = 0
    try {
      for (;;) {
        const body = await request(`jobs/${job.id}/rollback`, 'POST')
        if (
          !isRecord(body) ||
          typeof body.restored !== 'number' ||
          typeof body.skipped !== 'number' ||
          typeof body.matching_failed !== 'number' ||
          typeof body.remaining !== 'number'
        ) throw new Error('復元の結果を読み取れませんでした')
        restored += body.restored
        skipped += body.skipped
        matchingFailed += body.matching_failed
        if (body.remaining === 0) break
      }
      setRollbackMessage(
        `${restored} セッションを元に戻しました。再計算の後に更新があった ${skipped} セッションは戻していません。` +
        (matchingFailed > 0 ? `${matchingFailed} セッションは企業マッチングを更新できませんでした。` : ''),
      )
      void loadData()
    } catch (e) {
      setError(e instanceof Error ? e.message : '元に戻せませんでした')
    } finally {
      setProcessing(false)
    }
  }

  if (!platformReady) return null
  const progress = job && job.total_sessions > 0
    ? Math.min(100, Math.round(job.processed_sessions / job.total_sessions * 100))
    : 100

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.standard}>
      <AdminPageHeader
        title="ユーザースコア再計算"
        description="旧計算版のチャットスコアを、保存済みの質問・回答から再構築します。"
        backHref="/admin"
      />
      {error && (
        <Alert
          severity="error"
          sx={{ mb: 2 }}
          action={<Button color="inherit" size="small" onClick={() => void loadData()}>再読み込み</Button>}
        >
          {error}
        </Alert>
      )}
      {loading ? (
        <Stack alignItems="center" py={5}><CircularProgress /></Stack>
      ) : (
        <Stack spacing={2}>
          {preview && (
            <Paper variant="outlined" sx={{ p: 2 }}>
              <Typography variant="h6" gutterBottom>実行前の見積もり</Typography>
              <Typography>計算版: {preview.calculation_version}</Typography>
              <Typography>再計算可能なセッション: {preview.eligible_sessions} 件</Typography>
              <Typography>対象外の古いセッション: {preview.unsupported_sessions} 件</Typography>
              <Typography>LLM呼び出し上限見積もり: {preview.estimated_llm_calls} 回</Typography>
              <Typography>費用概算: ${preview.estimated_cost_usd.toFixed(4)} USD</Typography>
              {preview.eligible_sessions === 0 && (
                <Alert severity="info" sx={{ mt: 2 }}>
                  再計算が必要な対象はありません。
                </Alert>
              )}
              <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                {preview.cost_estimate_note}
              </Typography>
              {preview.unsupported_sessions > 0 && (
                <Alert severity="warning" sx={{ mt: 2 }}>
                  面接・書類評価が混在するセッションや元データがないスコアは、誤上書きを避けるため対象外です。
                </Alert>
              )}
              <Button
                variant="contained"
                sx={{ mt: 2 }}
                disabled={preview.eligible_sessions === 0 || job?.status === 'running' || job?.status === 'paused' || processing}
                onClick={() => setConfirmOpen(true)}
              >
                再計算を開始
              </Button>
              <Button
                variant="outlined"
                sx={{ mt: 2, ml: 1 }}
                disabled={preview.eligible_sessions === 0 || dryRunning || processing}
                onClick={() => void runDryRun()}
              >
                {dryRunning ? '確認中…' : '再計算後の値を確認'}
              </Button>
              <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                先頭の3セッションだけを再計算して、保存せずに現在の値と比べます。LLM を呼ぶので費用がかかります。
              </Typography>
              {dryRunText && (
                <Typography
                  component="pre"
                  variant="body2"
                  sx={{ mt: 1, p: 1, overflowX: 'auto', bgcolor: 'action.hover', fontFamily: 'monospace' }}
                >
                  {dryRunText}
                </Typography>
              )}
            </Paper>
          )}
          {job && (
            <Paper variant="outlined" sx={{ p: 2 }}>
              <Stack direction={{ xs: 'column', sm: 'row' }} justifyContent="space-between" spacing={1}>
                <Typography variant="h6">ジョブ #{job.id} — {JOB_STATUS_LABELS[job.status]}</Typography>
                <Typography variant="body2" color="text.secondary">版 {job.calculation_version}</Typography>
              </Stack>
              <LinearProgress variant="determinate" value={progress} sx={{ my: 2 }} />
              <Typography>
                {job.processed_sessions} / {job.total_sessions} セッション処理済み
                {job.failed_sessions > 0 && `（失敗 ${job.failed_sessions} 件）`}
              </Typography>
              {job.failed_sessions > 0 && job.last_error && (
                <Typography variant="body2" color="error" sx={{ mt: 1 }}>
                  直近の失敗: {FAILURE_LABELS[job.last_error] ?? job.last_error}
                </Typography>
              )}
              <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                費用概算: ${job.estimated_cost_usd.toFixed(4)} USD
              </Typography>
              {rollbackMessage && (
                <Alert severity="info" sx={{ mt: 2 }}>{rollbackMessage}</Alert>
              )}
              <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
                {job.status === 'running' && !processing && (
                  <Button variant="contained" onClick={continueJob}>
                    続行
                  </Button>
                )}
                {job.status === 'running' && (
                  <Button variant="outlined" onClick={() => void pauseJob()}>
                    一時停止
                  </Button>
                )}
                {job.status === 'paused' && (
                  <Button variant="contained" disabled={processing} onClick={() => void resumeJob()}>
                    再開
                  </Button>
                )}
                {job.status === 'paused' && (
                  <Button variant="outlined" color="error" disabled={processing} onClick={() => setCancelConfirmOpen(true)}>
                    中止
                  </Button>
                )}
                {(job.status === 'completed' || job.status === 'cancelled') && (
                  <Button variant="outlined" color="error" disabled={processing} onClick={() => setRollbackConfirmOpen(true)}>
                    元に戻す
                  </Button>
                )}
                {processing && <CircularProgress size={24} aria-label="再計算中" />}
              </Stack>
            </Paper>
          )}
        </Stack>
      )}
      <Dialog open={confirmOpen} onClose={() => setConfirmOpen(false)}>
        <DialogTitle>ユーザースコア再計算を開始しますか？</DialogTitle>
        <DialogContent>
          <Typography>
            {preview?.eligible_sessions ?? 0} セッションを再計算します。費用概算は
            ${preview?.estimated_cost_usd.toFixed(4) ?? '0.0000'} USDです。
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
            実行中は一時停止でき、再開後は未処理分から続行します。
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmOpen(false)}>キャンセル</Button>
          <Button variant="contained" onClick={() => void startJob()}>開始する</Button>
        </DialogActions>
      </Dialog>
      <Dialog open={cancelConfirmOpen} onClose={() => setCancelConfirmOpen(false)}>
        <DialogTitle>再計算を中止しますか？</DialogTitle>
        <DialogContent>
          <Typography>
            再計算済みのセッションは新しい値のまま残ります（中止の後に「元に戻す」で戻せます）。未処理のセッションは、次に開始するジョブの対象になります。
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCancelConfirmOpen(false)}>戻る</Button>
          <Button variant="contained" color="error" onClick={() => void cancelJob()}>中止する</Button>
        </DialogActions>
      </Dialog>
      <Dialog open={rollbackConfirmOpen} onClose={() => setRollbackConfirmOpen(false)}>
        <DialogTitle>再計算の前の値に戻しますか？</DialogTitle>
        <DialogContent>
          <Typography>
            このジョブが置き換えたスコアを元に戻し、企業マッチングも作り直します。再計算の後にチャット回答や履歴書・ES・面接の反映があったセッションは戻しません。
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setRollbackConfirmOpen(false)}>戻る</Button>
          <Button variant="contained" color="error" onClick={() => void rollbackJob()}>元に戻す</Button>
        </DialogActions>
      </Dialog>
    </PageContainer>
  )
}
