export const SCOUT_COOLDOWN_MS = 24 * 60 * 60 * 1000

export type ScoutStatus = 'sent' | 'viewed' | 'accepted' | 'declined'

export interface ScoutCandidate {
  id: number
  name: string
  school: string
}

export interface ScoutTemplate {
  id: number
  title: string
  body: string
  createdAt: string
}

export interface ScoutMessage {
  id: number
  userId: number
  studentName: string
  companyName: string
  templateId: number | null
  templateTitle: string
  message: string
  status: ScoutStatus
  createdAt: string
}

export interface ScoutState {
  companyName: string
  nextId: number
  templates: ScoutTemplate[]
  scouts: ScoutMessage[]
  blockedCompanyIds: string[]
}

const STORAGE_KEY = 'soc-mock-scout-state'

export const MOCK_CANDIDATES: ScoutCandidate[] = [
  { id: 101, name: '山田太郎', school: '情報セキュリティ大学' },
  { id: 102, name: '佐藤花子', school: 'IT専門学校' },
  { id: 103, name: '鈴木一郎', school: '情報セキュリティ大学' },
]

const DEFAULT_COMPANY = 'デモ株式会社'

function defaultTemplates(): ScoutTemplate[] {
  return [
    {
      id: 1,
      title: 'カジュアル面談のご案内',
      body: '{{学生名}} さん\n\n{{企業名}}の採用担当です。プロフィールを拝見し、ぜひ一度カジュアルにお話ししませんか。\nご都合のよい日時を返信いただけると助かります。',
      createdAt: '2026-09-01T00:00:00.000Z',
    },
    {
      id: 2,
      title: '選考のご案内',
      body: '{{学生名}} さん\n\n{{企業名}}です。ご経験を拝見し、次の選考にご案内したくご連絡しました。\nまずは30分程度、オンラインでお話しできればと思います。',
      createdAt: '2026-09-02T00:00:00.000Z',
    },
  ]
}

export function emptyScoutState(): ScoutState {
  return {
    companyName: DEFAULT_COMPANY,
    nextId: 3,
    templates: defaultTemplates(),
    scouts: [],
    blockedCompanyIds: [],
  }
}

export function interpolateScoutBody(
  body: string,
  vars: { studentName: string; companyName: string },
): string {
  return body
    .replaceAll('{{学生名}}', vars.studentName)
    .replaceAll('{{企業名}}', vars.companyName)
    .replaceAll('{{name}}', vars.studentName)
    .replaceAll('{{company}}', vars.companyName)
}

export function lastScoutToStudent(scouts: ScoutMessage[], userId: number): ScoutMessage | undefined {
  return scouts
    .filter((s) => s.userId === userId)
    .sort((a, b) => (a.createdAt < b.createdAt ? 1 : -1))[0]
}

export function cooldownRemainingMs(last: ScoutMessage | undefined, nowMs: number): number {
  if (!last) return 0
  const sentAt = Date.parse(last.createdAt)
  if (Number.isNaN(sentAt)) return 0
  return Math.max(0, sentAt + SCOUT_COOLDOWN_MS - nowMs)
}

export function formatCooldown(ms: number): string {
  const hours = Math.ceil(ms / (60 * 60 * 1000))
  if (hours >= 24) return '24時間'
  if (hours <= 1) return '1時間以内'
  return `あと約${hours}時間`
}

export const SCOUT_STATUS_LABEL: Record<ScoutStatus, string> = {
  sent: '未読',
  viewed: '既読',
  accepted: '承諾',
  declined: '辞退',
}

export function loadScoutState(): ScoutState {
  if (typeof window === 'undefined') return emptyScoutState()
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return emptyScoutState()
    const parsed = JSON.parse(raw) as Partial<ScoutState>
    return {
      companyName: parsed.companyName || DEFAULT_COMPANY,
      nextId: typeof parsed.nextId === 'number' ? parsed.nextId : 3,
      templates: Array.isArray(parsed.templates) ? parsed.templates : defaultTemplates(),
      scouts: Array.isArray(parsed.scouts) ? parsed.scouts : [],
      blockedCompanyIds: Array.isArray(parsed.blockedCompanyIds) ? parsed.blockedCompanyIds : [],
    }
  } catch {
    return emptyScoutState()
  }
}

export function saveScoutState(state: ScoutState): void {
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
}

export function createTemplate(state: ScoutState, title: string, body: string): ScoutState {
  const id = state.nextId
  return {
    ...state,
    nextId: id + 1,
    templates: [
      ...state.templates,
      { id, title: title.trim(), body: body.trim(), createdAt: new Date().toISOString() },
    ],
  }
}

export function updateTemplate(state: ScoutState, id: number, title: string, body: string): ScoutState {
  return {
    ...state,
    templates: state.templates.map((t) =>
      t.id === id ? { ...t, title: title.trim(), body: body.trim() } : t,
    ),
  }
}

export function deleteTemplate(state: ScoutState, id: number): ScoutState {
  return { ...state, templates: state.templates.filter((t) => t.id !== id) }
}

export function sendScout(
  state: ScoutState,
  input: { userId: number; studentName: string; template: ScoutTemplate; nowMs: number },
): { ok: true; state: ScoutState } | { ok: false; reason: 'cooldown' | 'blocked'; remainingMs?: number } {
  if (state.blockedCompanyIds.includes(state.companyName)) {
    return { ok: false, reason: 'blocked' }
  }
  const remaining = cooldownRemainingMs(lastScoutToStudent(state.scouts, input.userId), input.nowMs)
  if (remaining > 0) {
    return { ok: false, reason: 'cooldown', remainingMs: remaining }
  }
  const id = state.nextId
  const message: ScoutMessage = {
    id,
    userId: input.userId,
    studentName: input.studentName,
    companyName: state.companyName,
    templateId: input.template.id,
    templateTitle: input.template.title,
    message: interpolateScoutBody(input.template.body, {
      studentName: input.studentName,
      companyName: state.companyName,
    }),
    status: 'sent',
    createdAt: new Date(input.nowMs).toISOString(),
  }
  return {
    ok: true,
    state: { ...state, nextId: id + 1, scouts: [message, ...state.scouts] },
  }
}

export function markScoutViewed(state: ScoutState, id: number): ScoutState {
  return {
    ...state,
    scouts: state.scouts.map((s) => (s.id === id && s.status === 'sent' ? { ...s, status: 'viewed' } : s)),
  }
}

export function declineScout(state: ScoutState, id: number): ScoutState {
  return {
    ...state,
    scouts: state.scouts.map((s) => (s.id === id && s.status !== 'declined' ? { ...s, status: 'declined' } : s)),
  }
}

export function blockCompany(state: ScoutState, companyName: string): ScoutState {
  if (state.blockedCompanyIds.includes(companyName)) return state
  return { ...state, blockedCompanyIds: [...state.blockedCompanyIds, companyName] }
}
