import { authService } from '@/lib/auth'
import { fetchAndReadWithTimeout } from '@/lib/fetch-timeout'

/** 自己分析ポートフォリオ（#2030）で選べる内容。並びが出力の順番。 */
export const PORTFOLIO_SECTIONS = [
  { key: 'diagnosis', label: '適性の診断' },
  { key: 'interview', label: 'AI 面接' },
  { key: 'es', label: 'ES 添削' },
  { key: 'assessment', label: '適性検査' },
] as const

export type PortfolioSection = (typeof PORTFOLIO_SECTIONS)[number]['key']

export type CategoryScore = { category: string; score: number }

export type PortfolioData = {
  generated_at: string
  sections: PortfolioSection[]
  diagnosis?: { updated_at?: string; scores: CategoryScore[] } | null
  interviews?: {
    session_id: number
    created_at: string
    summary_text: string
    strengths: string[]
    improvements: string[]
  }[] | null
  es_reviews?: {
    id: number
    created_at: string
    question: string
    company_name?: string
    specificity_score: number
    star_score: number
    length_balance_score: number
    company_fit_score?: number
    feedback: string
  }[] | null
  assessments?: { created_at: string; title: string; scores: CategoryScore[] }[] | null
}

/**
 * timeoutMs を渡すと、本文の受信まで含めてその時間で打ち切る（ホームの帯のように、
 * ほかの取得と一緒に待つ呼び出しで、半開きの接続に引きずられないように）。
 */
export async function fetchPortfolio(sections: PortfolioSection[], timeoutMs?: number): Promise<PortfolioData> {
  const url = `/api/portfolio?sections=${encodeURIComponent(sections.join(','))}`
  const init: RequestInit = { headers: authService.getUserFetchHeaders(), cache: 'no-store' }
  const read = async (res: Response): Promise<PortfolioData> => {
    if (!res.ok) throw new Error(`portfolio ${res.status}`)
    return (await res.json()) as PortfolioData
  }
  if (timeoutMs !== undefined) return fetchAndReadWithTimeout(url, init, timeoutMs, read)
  return read(await fetch(url, init))
}

export function formatDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleDateString('ja-JP', { year: 'numeric', month: 'long', day: 'numeric' })
}
