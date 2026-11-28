import { z } from 'zod'

const sourceSchema = z.object({
  company_id: z.number().int().positive(),
  type: z.string(),
  title: z.string(),
  url: z.string().optional().default(''),
  as_of: z.string().optional().default(''),
  fetched_at: z.string().optional().default(''),
})

export const researchChatAnswerSchema = z.object({
  status: z.enum(['answered', 'not_found']),
  answer: z.string(),
  company_ids: z.array(z.number().int().positive()).nullable().transform((value) => value ?? []),
  sources: z.array(sourceSchema).nullable().transform((value) => value ?? []),
})

export type ResearchChatAnswer = z.infer<typeof researchChatAnswerSchema>
export type ResearchChatSource = z.infer<typeof sourceSchema>

export const MAX_RESEARCH_QUESTION_LENGTH = 500

export class ResearchChatError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message)
    this.name = 'ResearchChatError'
  }
}

/** 企業研究AIに質問する。Browser → BFF → Backend。 */
export async function askResearchChat(
  companyId: number,
  question: string,
  options: { compareWith?: number; signal?: AbortSignal } = {},
): Promise<ResearchChatAnswer> {
  const body: { question: string; compare_with?: number } = { question: question.trim() }
  if (options.compareWith && options.compareWith > 0 && options.compareWith !== companyId) {
    body.compare_with = options.compareWith
  }
  const response = await fetch(`/api/companies/${companyId}/research-chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    cache: 'no-store',
    signal: options.signal,
    body: JSON.stringify(body),
  })
  const raw = (await response.json().catch(() => ({}))) as { error?: string; detail?: string; status?: string; answer?: string }
  if (!response.ok) {
    const message =
      (typeof raw.error === 'string' && raw.error.trim()) ||
      (typeof raw.detail === 'string' && raw.detail.trim()) ||
      (response.status === 429
        ? '今日の質問回数の上限に達しました。明日また利用できます。'
        : response.status === 503
          ? '回答を作れませんでした。時間をおいて再度お試しください。'
          : '質問を送れませんでした。もう一度お試しください。')
    throw new ResearchChatError(message, response.status)
  }
  return researchChatAnswerSchema.parse(raw)
}
