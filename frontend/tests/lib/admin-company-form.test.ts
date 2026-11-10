import { applyInfoPayload } from '@/lib/admin/company-form'

function setters(setPromptVersion: (v: string) => void) {
  const noop = () => {}
  return {
    setDescription: noop, setIndustry: noop, setLocation: noop, setWebsiteUrl: noop,
    setFoundedYear: noop, setEmployeeCount: noop, setMainBusiness: noop, setCulture: noop,
    setWorkStyle: noop, setTechStack: noop, setWelfareDetails: noop, setSourceType: noop,
    setSourceUrl: noop, setLastModelUsed: noop, setLastFetchConfidence: noop, setPromptVersion,
  }
}

describe('applyInfoPayload の prompt_version', () => {
  it('AIプレビューの版をフォームへ渡す', () => {
    const setPromptVersion = jest.fn()
    applyInfoPayload({ prompt_version: 'company-t1' }, setters(setPromptVersion))
    expect(setPromptVersion).toHaveBeenCalledWith('company-t1')
  })

  it('版が無いプレビューでは前回の版を残さない', () => {
    const setPromptVersion = jest.fn()
    applyInfoPayload({ description: '概要' }, setters(setPromptVersion))
    expect(setPromptVersion).toHaveBeenCalledWith('')
  })
})
