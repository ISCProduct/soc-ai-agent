/**
 * @jest-environment jsdom
 */
import { act, fireEvent, render, screen } from '@testing-library/react'
import InterviewSummary from '@/app/interview/components/InterviewSummary'
import { interviewApi } from '@/lib/interview'
import type { InterviewDetail, InterviewReport, InterviewUtterance } from '@/lib/interview'

describe('InterviewSummary', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  // スコアが算出できなかったレポート（#795 でスコアだけ捨てる経路がある）。
  // 空オブジェクトは truthy なので、件数を見ないと 0/0 で「NaN / 5」と表示される。
  it.each([
    ['空オブジェクト', '{}'],
    ['空文字', ''],
    ['null', 'null'],
  ])('スコアが%sでも講評を表示し、NaN を出さない', (_name, scoresJson) => {
    const report: InterviewReport = {
      session_id: 1,
      summary_text: '落ち着いて回答できていました。',
      scores_json: scoresJson,
      evidence_json: '',
      strengths_json: JSON.stringify(['結論から話せる']),
      improvements_json: JSON.stringify(['数値を添える']),
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    }

    render(<InterviewSummary report={report} />)

    // 講評は届く
    expect(screen.getByText('落ち着いて回答できていました。')).toBeInTheDocument()
    // スコア欄は出さない
    expect(screen.queryByText('評価の内訳')).not.toBeInTheDocument()
    // NaN を画面に出さない
    expect(document.body.textContent).not.toMatch(/NaN/)
  })

  it('面接サマリーとスコア情報を表示する', () => {
    const report: InterviewReport = {
      session_id: 1,
      summary_text: '総合的に落ち着いて回答できています。',
      scores_json: JSON.stringify({
        logic: 4.5,
        communication: 4.0,
      }),
      evidence_json: JSON.stringify({
        logic: '結論から先に説明できていました。',
      }),
      strengths_json: JSON.stringify(['結論ファーストで回答できる']),
      improvements_json: JSON.stringify(['具体例を増やす']),
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    }

    render(<InterviewSummary report={report} />)

    expect(screen.getByText('総合評価')).toBeInTheDocument()
    expect(screen.getByText('総合的に落ち着いて回答できています。')).toBeInTheDocument()
    expect(screen.getByText('評価の内訳')).toBeInTheDocument()
    expect(screen.getByText('論理性')).toBeInTheDocument()
    expect(screen.getByText('4.5 / 5')).toBeInTheDocument()
    expect(screen.getByText('「結論から先に説明できていました。」')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '論理性の根拠発言を見る' })).not.toBeInTheDocument()
  })

  it('評点の引用から該当する発話ログを開く', () => {
    const report: InterviewReport = {
      session_id: 7,
      summary_text: '要点を伝えられています。',
      scores_json: JSON.stringify({ logic: 4 }),
      evidence_json: JSON.stringify({ logic: '私は結論から説明しました。' }),
      evidence_utterance_ids_json: JSON.stringify({ logic: 42 }),
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    }
    const utterances: InterviewUtterance[] = [
      {
        id: 42,
        session_id: 7,
        role: 'user',
        text: '私は結論から説明しました。',
        created_at: '2026-01-01T00:00:00Z',
      },
    ]

    render(<InterviewSummary report={report} utterances={utterances} />)

    fireEvent.click(screen.getByRole('button', { name: '論理性の根拠発言を見る' }))

    expect(screen.getByRole('region', { name: '面接発話ログ' })).toBeInTheDocument()
    expect(screen.getByText('私は結論から説明しました。')).toBeInTheDocument()
  })

  it('ライブレポートでは必要になったときだけ発話ログを取得する', async () => {
    const report: InterviewReport = {
      session_id: 7,
      summary_text: '要点を伝えられています。',
      scores_json: JSON.stringify({ logic: 4 }),
      evidence_json: JSON.stringify({ logic: '私は結論から説明しました。' }),
      evidence_utterance_ids_json: JSON.stringify({ logic: 42 }),
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    }
    const utterances: InterviewUtterance[] = [
      {
        id: 42,
        session_id: 7,
        role: 'user',
        text: '私は結論から説明しました。',
        created_at: '2026-01-01T00:00:00Z',
      },
    ]
    const detail: InterviewDetail = {
      session: {
        id: 7,
        user_id: 1,
        status: 'finished',
        language: 'ja',
        estimated_cost_usd: 0,
        template_version: '',
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
      utterances,
    }
    const getDetail = jest.spyOn(interviewApi, 'getDetail').mockResolvedValue(detail)
    jest.spyOn(interviewApi, 'getPhraseSuggestions').mockResolvedValue([])

    render(<InterviewSummary report={report} userId={1} />)

    fireEvent.click(screen.getByRole('button', { name: '論理性の根拠発言を見る' }))

    expect(await screen.findByRole('region', { name: '面接発話ログ' })).toBeInTheDocument()
    expect(screen.getByText('私は結論から説明しました。')).toBeInTheDocument()
    expect(getDetail).toHaveBeenCalledWith(7, 1)
  })

  it('別セッションに切り替えた後に届いた古い発話ログを表示しない', async () => {
    const makeReport = (sessionId: number): InterviewReport => ({
      session_id: sessionId,
      summary_text: '要点を伝えられています。',
      scores_json: JSON.stringify({ logic: 4 }),
      evidence_json: JSON.stringify({ logic: '根拠となる発言です。' }),
      evidence_utterance_ids_json: JSON.stringify({ logic: sessionId }),
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    })
    const makeDetail = (sessionId: number, text: string): InterviewDetail => ({
      session: {
        id: sessionId,
        user_id: 1,
        status: 'finished',
        language: 'ja',
        estimated_cost_usd: 0,
        template_version: '',
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
      utterances: [{
        id: sessionId,
        session_id: sessionId,
        role: 'user',
        text,
        created_at: '2026-01-01T00:00:00Z',
      }],
    })
    const oldDetail = makeDetail(7, '古いセッションの発言')
    let resolveOldDetail: (detail: InterviewDetail) => void = () => {
      throw new Error('Old detail request has not been initialized')
    }
    const pendingOldDetail = new Promise<InterviewDetail>((resolve) => {
      resolveOldDetail = resolve
    })
    jest.spyOn(interviewApi, 'getDetail').mockImplementation((sessionId) => (
      sessionId === 7 ? pendingOldDetail : Promise.resolve(makeDetail(8, '新しいセッションの発言'))
    ))
    jest.spyOn(interviewApi, 'getPhraseSuggestions').mockResolvedValue([])

    const { rerender } = render(<InterviewSummary report={makeReport(7)} userId={1} />)
    fireEvent.click(screen.getByRole('button', { name: '論理性の根拠発言を見る' }))

    rerender(<InterviewSummary report={makeReport(8)} userId={1} />)
    fireEvent.click(screen.getByRole('button', { name: '論理性の根拠発言を見る' }))

    expect(await screen.findByText('新しいセッションの発言')).toBeInTheDocument()
    await act(async () => {
      resolveOldDetail(oldDetail)
      await pendingOldDetail
    })
    expect(screen.queryByText('古いセッションの発言')).not.toBeInTheDocument()
  })
})
