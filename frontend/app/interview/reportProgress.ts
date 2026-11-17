/**
 * 面接レポート生成の進捗（段階・%）の表示ロジック。
 * バックエンドが GET /api/interviews/{id} の session に載せる report_* を、
 * 画面に出す%・停滞判定に変換する。UI / hook から分離し単体テスト可能にする（SOCAIAGENT-652）。
 */

export type ReportStage = 'queued' | 'loading' | 'generating' | 'saving' | 'done' | 'failed'

export type ReportProgress = {
  stage: ReportStage
  /** サーバーが保持する値（0〜100）。generating の間は 10 のまま動かない */
  percent: number
  failedReason?: string
}

export type ReportProgressView = {
  stage: ReportStage
  /** 画面に出す%（整数・後退しない） */
  percent: number
  /** 同じ段階が閾値を超えて続いている（「時間がかかっています」を出す） */
  stalled: boolean
}

/** 画面に並べる段階。done / failed は行ではなく状態として扱う */
export const REPORT_STAGE_STEPS: { stage: ReportStage; label: string }[] = [
  { stage: 'queued', label: '順番待ち' },
  { stage: 'loading', label: '面接の内容の読み込み' },
  { stage: 'generating', label: '回答の分析' },
  { stage: 'saving', label: 'レポートの保存' },
]

const STAGES: readonly string[] = ['queued', 'loading', 'generating', 'saving', 'done', 'failed']

/** generating 中の%を 10→85 に寄せる所要時間（暫定。実測後に見直す） */
const GENERATING_FROM = 10
const GENERATING_TO = 85
const GENERATING_INTERPOLATE_MS = 120_000

/** 停滞とみなす時間。generating は生成ジョブの上限（10分）、他は暫定 60 秒 */
const STALL_MS: Record<ReportStage, number> = {
  queued: 60_000,
  loading: 60_000,
  generating: 10 * 60_000,
  saving: 60_000,
  done: Infinity,
  failed: Infinity,
}

/**
 * セッション詳細から進捗を取り出す。
 * report_stage が無い（旧バックエンド）・未知の値のときは null を返し、呼び出し側は従来表示にする。
 */
export function parseReportProgress(session: unknown): ReportProgress | null {
  if (typeof session !== 'object' || session === null) return null
  const s = session as Record<string, unknown>
  const stage = s.report_stage
  if (typeof stage !== 'string' || !STAGES.includes(stage)) return null
  const raw = typeof s.report_percent === 'number' ? s.report_percent : 0
  return {
    stage: stage as ReportStage,
    percent: Math.min(100, Math.max(0, raw)),
    failedReason: typeof s.report_failed_reason === 'string' ? s.report_failed_reason : undefined,
  }
}

export type ProgressTracker = {
  stage: ReportStage | null
  /** この段階を初めて観測した時刻（ms） */
  sinceMs: number
  /** これまでに表示した最大の% */
  shown: number
}

export const INITIAL_PROGRESS_TRACKER: ProgressTracker = { stage: null, sinceMs: 0, shown: 0 }

/**
 * ポーリング1回ぶんの進捗を、表示用の値に進める。
 * - %は後退させない（サーバー値の取得が揺れても戻らない）
 * - done 以外は 99 を上限にする（100% 表示はレポート到着後のみ）
 * - 段階の経過時間は、サーバー時刻ではなくクライアントが観測した時刻で測る（時計ずれ対策）
 */
export function advanceProgress(
  tracker: ProgressTracker,
  progress: ReportProgress,
  nowMs: number,
): { tracker: ProgressTracker; view: ReportProgressView } {
  const sinceMs = tracker.stage === progress.stage ? tracker.sinceMs : nowMs
  const elapsed = nowMs - sinceMs

  let target = progress.percent
  if (progress.stage === 'generating') {
    const ratio = Math.min(1, elapsed / GENERATING_INTERPOLATE_MS)
    target = Math.max(target, GENERATING_FROM + (GENERATING_TO - GENERATING_FROM) * ratio)
  }
  const cap = progress.stage === 'done' ? 100 : 99
  const percent = Math.min(cap, Math.max(tracker.shown, Math.floor(target)))

  return {
    tracker: { stage: progress.stage, sinceMs, shown: percent },
    view: { stage: progress.stage, percent, stalled: elapsed >= STALL_MS[progress.stage] },
  }
}
