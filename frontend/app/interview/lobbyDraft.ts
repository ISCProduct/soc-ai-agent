/**
 * 面接ロビーの下書き（企業・職種）を sessionStorage に保持する。
 * リロード後も会社選択に戻さずロビーを復元するため。
 */

import type { InterviewCompany, Position } from './types'
import { POSITIONS } from './constants'

export const INTERVIEW_LOBBY_DRAFT_KEY = 'interview_lobby_draft_v1'

export type InterviewLobbyDraft = {
  company: InterviewCompany
  positionId: string
  positionCategory: 'general' | 'sier'
  savedAt: number
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
}

export function parseInterviewLobbyDraft(raw: string | null): InterviewLobbyDraft | null {
  if (!raw) return null
  try {
    const data: unknown = JSON.parse(raw)
    if (!isRecord(data)) return null
    const company = data.company
    if (!isRecord(company) || typeof company.name !== 'string' || !company.name.trim()) {
      return null
    }
    const id = typeof company.id === 'number' && Number.isFinite(company.id) ? company.id : 0
    const positionId = typeof data.positionId === 'string' ? data.positionId : POSITIONS[0].id
    const positionCategory =
      data.positionCategory === 'sier' || data.positionCategory === 'general'
        ? data.positionCategory
        : 'general'
    return {
      company: {
        id,
        name: company.name.trim(),
        industry: typeof company.industry === 'string' ? company.industry : undefined,
        description: typeof company.description === 'string' ? company.description : undefined,
        location: typeof company.location === 'string' ? company.location : undefined,
      },
      positionId,
      positionCategory,
      savedAt: typeof data.savedAt === 'number' ? data.savedAt : Date.now(),
    }
  } catch {
    return null
  }
}

export function loadInterviewLobbyDraft(
  storage: Pick<Storage, 'getItem'> | null | undefined = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
): InterviewLobbyDraft | null {
  if (!storage) return null
  try {
    return parseInterviewLobbyDraft(storage.getItem(INTERVIEW_LOBBY_DRAFT_KEY))
  } catch {
    return null
  }
}

export function saveInterviewLobbyDraft(
  draft: Omit<InterviewLobbyDraft, 'savedAt'>,
  storage: Pick<Storage, 'setItem'> | null | undefined = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
): void {
  if (!storage) return
  try {
    const payload: InterviewLobbyDraft = { ...draft, savedAt: Date.now() }
    storage.setItem(INTERVIEW_LOBBY_DRAFT_KEY, JSON.stringify(payload))
  } catch {
    // private mode 等では無視
  }
}

export function clearInterviewLobbyDraft(
  storage: Pick<Storage, 'removeItem'> | null | undefined = typeof sessionStorage !== 'undefined' ? sessionStorage : null,
): void {
  if (!storage) return
  try {
    storage.removeItem(INTERVIEW_LOBBY_DRAFT_KEY)
  } catch {
    // ignore
  }
}

export function resolvePositionFromDraft(
  positionId: string,
  category: 'general' | 'sier',
): Position {
  const found = POSITIONS.find((p) => p.id === positionId)
  if (found) return found
  return POSITIONS.find((p) => p.category === category) ?? POSITIONS[0]
}
