import { BACKEND_URL } from './backend-url'

const DEFAULT_INTERVIEW_MAX_MINUTES = 10
const DEFAULT_INTERVIEW_QUESTION_DURATION_SECONDS = 180

export type InterviewSession = {
  id: number
  user_id: number
  status: string
  language: string
  started_at?: string
  ended_at?: string
  estimated_cost_usd: number
  template_version: string
  created_at: string
  updated_at: string
}

export const INTERVIEW_LANGUAGES = [
  { code: 'ja', label: '日本語' },
  { code: 'en', label: 'English' },
  { code: 'zh', label: '中文（简体）' },
  { code: 'ko', label: '한국어' },
  { code: 'fr', label: 'Français' },
  { code: 'es', label: 'Español' },
  { code: 'de', label: 'Deutsch' },
  { code: 'pt', label: 'Português' },
  { code: 'it', label: 'Italiano' },
  { code: 'ar', label: 'العربية' },
  { code: 'ru', label: 'Русский' },
  { code: 'hi', label: 'हिन्दी' },
  { code: 'th', label: 'ภาษาไทย' },
  { code: 'vi', label: 'Tiếng Việt' },
  { code: 'id', label: 'Bahasa Indonesia' },
  { code: 'tr', label: 'Türkçe' },
] as const

export type InterviewLanguageCode = (typeof INTERVIEW_LANGUAGES)[number]['code'] | string

export type InterviewUtterance = {
  id: number
  session_id: number
  role: 'user' | 'ai'
  text: string
  created_at: string
}

export type TeacherReport = {
  overall_comment: string
  detailed_evidence: Record<string, string>
  coaching_points: string[]
  strengths_for_teacher: string[]
  next_steps: string[]
}

export type InterviewReport = {
  session_id: number
  summary_text: string
  scores_json: string
  evidence_json: string
  strengths_json?: string
  improvements_json?: string
  teacher_report_json?: string  // 教員のみ返却
  created_at: string
  updated_at: string
}

export type InterviewDetail = {
  session: InterviewSession
  utterances: InterviewUtterance[]
  report?: InterviewReport
}

export type InterviewTrendPoint = {
  session_id: number
  created_at: string
  logic: number | null
  specificity: number | null
  ownership: number | null
  communication: number | null
  enthusiasm: number | null
}

export type PhraseSuggestion = {
  original: string
  suggestions: string[]
}

export const interviewApi = {
  async createSession(userId: number, language = 'ja', interviewerGender?: string): Promise<InterviewSession> {
    const res = await fetch(`${BACKEND_URL}/api/interviews`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: userId, language, interviewer_gender: interviewerGender }),
    })
    if (!res.ok) throw new Error(await res.text())
    return res.json()
  },

  async startSession(sessionId: number, userId: number): Promise<InterviewSession> {
    const res = await fetch(`${BACKEND_URL}/api/interviews/${sessionId}/start`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: userId }),
    })
    if (!res.ok) throw new Error(await res.text())
    return res.json()
  },

  async finishSession(sessionId: number, userId: number): Promise<InterviewSession> {
    const res = await fetch(`${BACKEND_URL}/api/interviews/${sessionId}/finish`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: userId }),
    })
    if (!res.ok) throw new Error(await res.text())
    return res.json()
  },

  async saveUtterance(sessionId: number, userId: number, role: 'user' | 'ai', text: string): Promise<void> {
    const res = await fetch(`${BACKEND_URL}/api/interviews/${sessionId}/utterances`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: userId, role, text }),
    })
    if (!res.ok) throw new Error(await res.text())
  },

  async getDetail(sessionId: number, userId: number, role?: string): Promise<InterviewDetail> {
    const roleParam = role ? `&role=${role}` : ''
    const res = await fetch(`${BACKEND_URL}/api/interviews/${sessionId}?user_id=${userId}${roleParam}`)
    if (!res.ok) throw new Error(await res.text())
    return res.json()
  },

  async getReport(sessionId: number, userId: number): Promise<InterviewReport | null> {
    const res = await fetch(`${BACKEND_URL}/api/interviews/${sessionId}/report?user_id=${userId}`)
    if (res.status === 404) return null
    if (!res.ok) throw new Error(await res.text())
    return res.json()
  },

  async listSessions(userId: number, page = 1, limit = 20): Promise<{ sessions: InterviewSession[]; total: number }> {
    const res = await fetch(`${BACKEND_URL}/api/interviews?user_id=${userId}&page=${page}&limit=${limit}`)
    if (!res.ok) throw new Error(await res.text())
    return res.json()
  },

  async createRealtimeToken(userId: number, interviewId: number): Promise<string> {
    const res = await fetch(`${BACKEND_URL}/api/realtime/token`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: userId, interview_id: interviewId }),
    })
    if (!res.ok) throw new Error(await res.text())
    const data = await res.json()
    return data.client_secret
  },

  async getPhraseSuggestions(sessionId: number, userId: number): Promise<PhraseSuggestion[]> {
    const res = await fetch(`${BACKEND_URL}/api/interviews/${sessionId}/phrase-suggestions?user_id=${userId}`)
    if (!res.ok) throw new Error(await res.text())
    const data = await res.json()
    return data.suggestions as PhraseSuggestion[]
  },

  async getTrend(userId: number, limit = 0): Promise<InterviewTrendPoint[]> {
    const params = limit > 0 ? `?user_id=${userId}&limit=${limit}` : `?user_id=${userId}`
    const res = await fetch(`${BACKEND_URL}/api/interviews/trend${params}`)
    if (!res.ok) throw new Error(await res.text())
    const data = await res.json()
    return data.points as InterviewTrendPoint[]
  },

  async sendReportEmail(sessionId: number, userId: number): Promise<{ message: string }> {
    const res = await fetch(`${BACKEND_URL}/api/interviews/${sessionId}/send-report`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: userId }),
    })
    if (!res.ok) throw new Error(await res.text())
    return res.json()
  },

  uploadVideo(
    sessionId: number,
    userId: number,
    blob: Blob,
    onProgress?: (percent: number) => void,
  ): Promise<{ video_id: number; status: string }> {
    return new Promise((resolve, reject) => {
      const form = new FormData()
      form.append('user_id', String(userId))
      form.append('video', blob, `interview_${sessionId}.webm`)

      const xhr = new XMLHttpRequest()
      xhr.open('POST', `${BACKEND_URL}/api/interviews/${sessionId}/upload-video`)

      if (onProgress) {
        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) {
            onProgress(Math.round((e.loaded / e.total) * 100))
          }
        }
      }

      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve(JSON.parse(xhr.responseText))
        } else {
          reject(new Error(xhr.responseText || `アップロードに失敗しました (HTTP ${xhr.status})`))
        }
      }
      xhr.onerror = () => reject(new Error('ネットワークエラーが発生しました。接続を確認してください'))
      xhr.ontimeout = () => reject(new Error('アップロードがタイムアウトしました'))
      xhr.timeout = 30 * 60 * 1000 // 30分
      xhr.send(form)
    })
  },
}

export const interviewLimits = {
  maxMinutes: Number(process.env.NEXT_PUBLIC_INTERVIEW_MAX_MINUTES || DEFAULT_INTERVIEW_MAX_MINUTES),
  questionDurationSeconds: Number(process.env.NEXT_PUBLIC_INTERVIEW_QUESTION_DURATION_SECONDS || DEFAULT_INTERVIEW_QUESTION_DURATION_SECONDS),
}
