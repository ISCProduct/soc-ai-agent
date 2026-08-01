/** 企業管理フォームの共通ロジック */

export const WORK_STYLE_OPTIONS = ['リモート', 'ハイブリッド', 'オフィス'] as const

export type CompanyInfoSetters = {
  setDescription: (v: string) => void
  setIndustry: (v: string) => void
  setLocation: (v: string) => void
  setWebsiteUrl: (v: string) => void
  setFoundedYear: (v: string) => void
  setEmployeeCount: (v: string) => void
  setMainBusiness: (v: string) => void
  setCulture: (v: string) => void
  setWorkStyle: (v: string) => void
  setTechStack: (v: string) => void
  setWelfareDetails: (v: string) => void
  setSourceType: (v: string) => void
  setSourceUrl: (v: string) => void
  setLastModelUsed: (v: string) => void
  setLastFetchConfidence: (v: string) => void
}

/**
 * AI取得結果やAPI応答のペイロードをフォームのstateに反映する。
 * 空文字列・ゼロ値のフィールドは上書きしない。
 */
export function applyInfoPayload(
  data: Record<string, unknown>,
  setters: CompanyInfoSetters,
) {
  const s = (key: string, setter: (v: string) => void) => {
    if (typeof data[key] === 'string' && data[key]) setter(data[key] as string)
  }
  const n = (key: string, setter: (v: string) => void) => {
    if (typeof data[key] === 'number' && data[key]) setter(String(data[key]))
  }

  s('description', setters.setDescription)
  s('industry', setters.setIndustry)
  s('location', setters.setLocation)
  s('website_url', setters.setWebsiteUrl)
  n('founded_year', setters.setFoundedYear)
  n('employee_count', setters.setEmployeeCount)
  s('main_business', setters.setMainBusiness)
  s('culture', setters.setCulture)
  s('work_style', setters.setWorkStyle)
  s('tech_stack', setters.setTechStack)
  s('welfare_details', setters.setWelfareDetails)
  s('source', setters.setSourceType)
  s('source_url', setters.setSourceUrl)
  s('model_used', setters.setLastModelUsed)
  s('confidence', setters.setLastFetchConfidence)
}
