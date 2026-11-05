/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ESRewritePage from '@/app/es-rewrite/page-content'

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn() }),
  useSearchParams: () => new URLSearchParams('company_name=%E3%82%B5%E3%83%B3%E3%83%97%E3%83%AB%E6%A0%AA%E5%BC%8F%E4%BC%9A%E7%A4%BE'),
}))

describe('ESRewritePage', () => {
  it('company_nameクエリで志望企業名欄がプリフィルされる（結果カードからのES CTA導線）', () => {
    render(<ESRewritePage />)

    expect(screen.getByLabelText(/志望企業名/)).toHaveValue('サンプル株式会社')
  })
})

describe('ESRewritePage エラー表示 (#1015)', () => {
  beforeEach(() => {
    global.fetch = jest.fn()
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('プロキシがJSONエラーを返した場合、日本語の短いメッセージを表示する（生JSONは見せない）', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: false,
      json: async () => ({ error: 'ログインの有効期限が切れました。再度ログインしてください。', status: 401 }),
    })

    render(<ESRewritePage />)

    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: '元の文章です。' },
    })
    fireEvent.click(screen.getByRole('button', { name: '書き直す' }))

    expect(
      await screen.findByText('ログインの有効期限が切れました。再度ログインしてください。'),
    ).toBeInTheDocument()
    expect(screen.queryByText(/^\{.*"error"/)).not.toBeInTheDocument()
  })

  it('プロキシがJSONを返さない場合はフォールバックの日本語文言を表示する', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: false,
      json: async () => {
        throw new Error('not json')
      },
    })

    render(<ESRewritePage />)

    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: '元の文章です。' },
    })
    fireEvent.click(screen.getByRole('button', { name: '書き直す' }))

    await waitFor(() => {
      expect(screen.getByText('リライトに失敗しました。再試行してください。')).toBeInTheDocument()
    })
  })
})


describe('ESRewritePage 企業適合性の表示 (#1524)', () => {
  const REVIEW_BASE = {
    specificity_score: 7,
    star_score: 6,
    length_balance_score: 5,
    feedback: 'フィードバック本文',
    improved_text: '改善後の文章',
  }

  const review = async (result: Record<string, unknown>, company?: string) => {
    ;(global.fetch as jest.Mock).mockResolvedValue({ ok: true, json: async () => result })
    render(<ESRewritePage />)
    fireEvent.click(screen.getByRole('button', { name: 'ES添削' }))
    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: '元の文章です。' },
    })
    if (company !== undefined) {
      fireEvent.change(screen.getByLabelText(/志望企業名/), { target: { value: company } })
    }
    fireEvent.click(screen.getByRole('button', { name: '添削する' }))
    await screen.findByText('添削スコア')
  }

  beforeEach(() => {
    global.fetch = jest.fn()
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('企業情報を取得できなかったときは企業適合性を出さず、理由と次の操作を案内する', async () => {
    await review(
      { ...REVIEW_BASE, company_fit_score: null, company_strategy: null, company_context_source: 'none' },
      '株式会社サイバーエージェント',
    )

    // 0/10 に見える空のバーを出さない（低評価と誤解させるため）
    expect(screen.queryByText('企業適合性')).not.toBeInTheDocument()
    expect(
      screen.getByText(/企業情報を取得できなかったため、企業適合性は評価していません/),
    ).toBeInTheDocument()
    expect(screen.getByText(/株式会社サイバーエージェント/)).toBeInTheDocument()
    expect(screen.queryByText(/への対策アドバイス/)).not.toBeInTheDocument()
    // 他の項目は通常どおり評価されている
    expect(screen.getByText('具体性')).toBeInTheDocument()
  })

  it('企業名未入力のときは入力を促す', async () => {
    await review(
      { ...REVIEW_BASE, company_fit_score: null, company_strategy: null, company_context_source: 'none' },
      '',
    )

    expect(screen.getByText(/企業名を入力して添削すると、企業適合性も評価します/)).toBeInTheDocument()
    expect(screen.queryByText(/企業情報を取得できなかったため/)).not.toBeInTheDocument()
  })

  it('企業情報を取得できたときは企業適合性と対策アドバイスを表示する', async () => {
    await review(
      {
        ...REVIEW_BASE,
        company_fit_score: 8,
        company_strategy: '求める人物像に沿って準備しましょう。',
        company_context_source: 'web_search',
      },
      '株式会社サイバーエージェント',
    )

    expect(screen.getByText('企業適合性')).toBeInTheDocument()
    expect(screen.getByText('8 / 10')).toBeInTheDocument()
    expect(screen.getByText(/への対策アドバイス/)).toBeInTheDocument()
    expect(screen.queryByText(/企業情報を取得できなかったため/)).not.toBeInTheDocument()

    // 添削後に入力欄だけ書き換えても、結果カードは添削に使った企業名を出し続ける
    fireEvent.change(screen.getByLabelText(/志望企業名/), { target: { value: '別の会社' } })
    expect(screen.getByText('🏢 株式会社サイバーエージェントへの対策アドバイス')).toBeInTheDocument()
    expect(screen.queryByText(/別の会社への対策アドバイス/)).not.toBeInTheDocument()
  })

  it('企業情報は取得できたが点数だけ出せないときは「見つかりませんでした」と言わない (H-2)', async () => {
    // _clamp_company_fit がモデルの非数値回答を null にするため、
    // company_context_source が none 以外でも fit が null になるレスポンスが実在する
    await review(
      {
        ...REVIEW_BASE,
        company_fit_score: null,
        company_strategy: '企業情報に基づく対策',
        company_context_source: 'web_search',
      },
      '株式会社テスト',
    )

    expect(screen.getByText(/今回は企業適合性の点数を算出できませんでした/)).toBeInTheDocument()
    expect(screen.queryByText(/公開情報が見つかりませんでした/)).not.toBeInTheDocument()
    // 取得できた企業情報を根拠にした対策アドバイスは出したまま（説明と矛盾させない）
    expect(screen.getByText(/への対策アドバイス/)).toBeInTheDocument()
  })

  it('422は detail の案内文をそのまま表示する（BFFの一般文で上書きしない / #1521）', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: false,
      status: 422,
      json: async () => ({
        error: '処理に失敗しました。しばらくしてから再試行してください。',
        status: 422,
        detail: '文章が長すぎて添削できませんでした。文字数を減らしてお試しください。',
      }),
    })

    render(<ESRewritePage />)
    fireEvent.click(screen.getByRole('button', { name: 'ES添削' }))
    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: '長い文章です。' },
    })
    fireEvent.click(screen.getByRole('button', { name: '添削する' }))

    expect(
      await screen.findByText('文章が長すぎて添削できませんでした。文字数を減らしてお試しください。'),
    ).toBeInTheDocument()
  })
})


describe('ESRewritePage 422の生JSON表示防止 (#1015 / #1521)', () => {
  beforeEach(() => {
    global.fetch = jest.fn()
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('detailが文字列でない422では生の本文を表示せず、一般文にフォールバックする', async () => {
    const longText = 'あ'.repeat(10001)
    // api-proxy は本文に文字列 detail が無いとレスポンス本文そのものを detail に入れる
    const rawBody = JSON.stringify({
      detail: [
        {
          type: 'string_too_long',
          loc: ['body', 'es_text'],
          msg: 'String should have at most 10000 characters',
          input: longText,
        },
      ],
    })
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: false,
      status: 422,
      json: async () => ({
        error: '処理に失敗しました。しばらくしてから再試行してください。',
        status: 422,
        detail: rawBody,
      }),
    })

    render(<ESRewritePage />)
    fireEvent.click(screen.getByRole('button', { name: 'ES添削' }))
    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: '長い文章です。' },
    })
    fireEvent.click(screen.getByRole('button', { name: '添削する' }))

    expect(
      await screen.findByText('処理に失敗しました。しばらくしてから再試行してください。'),
    ).toBeInTheDocument()
    expect(screen.queryByText(/string_too_long/)).not.toBeInTheDocument()
    expect(screen.queryByText(new RegExp(longText.slice(0, 40)))).not.toBeInTheDocument()
  })

  it('ES本文の入力欄はAPIの上限(6,000字)で止める (#1564)', () => {
    render(<ESRewritePage />)

    // RAG側 models.ES_TEXT_MAX_LENGTH と同値。緩めると「送れるが必ず422」が復活する
    expect(screen.getByPlaceholderText(/チームで開発した経験があります/)).toHaveAttribute(
      'maxlength',
      '6000',
    )
  })
})

describe('ESRewritePage 設問の文字数上限 (#1523)', () => {
  beforeEach(() => {
    global.fetch = jest.fn()
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  const setup = (tab: 'ES添削' | 'ESリライト（STAR法）') => {
    render(<ESRewritePage />)
    fireEvent.click(screen.getByRole('button', { name: tab }))
    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: '元の文章です。' },
    })
  }

  const lastRequestBody = (): Record<string, unknown> => {
    const calls = (global.fetch as jest.Mock).mock.calls
    const init = calls[calls.length - 1][1] as { body: string }
    return JSON.parse(init.body) as Record<string, unknown>
  }

  it('入力した上限とモードを添削リクエストへ載せる', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      json: async () => ({
        specificity_score: 7, star_score: 6, length_balance_score: 5,
        feedback: 'f', improved_text: 'あ'.repeat(390),
        improved_text_length: 390, char_limit_satisfied: true,
      }),
    })
    setup('ES添削')
    fireEvent.change(screen.getByLabelText(/設問の文字数上限/), { target: { value: '400' } })
    fireEvent.click(screen.getByRole('button', { name: '400字程度' }))
    fireEvent.click(screen.getByRole('button', { name: '添削する' }))

    await screen.findByText('添削スコア')
    expect(lastRequestBody()).toMatchObject({ char_limit: 400, char_limit_mode: 'around' })
    expect(screen.getByText(/390 \/ 400 文字/)).toBeInTheDocument()
  })

  it('リライトでも同じ上限を送り、字数を表示する（同じ経路に統合済み / #1533）', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      json: async () => ({
        rewritten_text: 'あ'.repeat(380),
        star: { situation: 'S', task: 'T', action: 'A', result: 'R' },
        improved_text_length: 380,
        char_limit_satisfied: true,
      }),
    })
    setup('ESリライト（STAR法）')
    fireEvent.change(screen.getByLabelText(/設問の文字数上限/), { target: { value: '400' } })
    fireEvent.click(screen.getByRole('button', { name: '書き直す' }))

    await screen.findByText('リライト後')
    expect(lastRequestBody()).toMatchObject({ char_limit: 400, char_limit_mode: 'within' })
    expect(screen.getByText(/380 \/ 400 文字/)).toBeInTheDocument()
    // 統合後もSTAR分解は表示される
    expect(screen.getByText('STAR法 分解')).toBeInTheDocument()
  })

  it('収まらなかったときは切り詰めず、収まらなかったことを伝える', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      json: async () => ({
        specificity_score: 7, star_score: 6, length_balance_score: 5,
        feedback: 'f', improved_text: 'あ'.repeat(520),
        improved_text_length: 520, char_limit_satisfied: false,
      }),
    })
    setup('ES添削')
    fireEvent.change(screen.getByLabelText(/設問の文字数上限/), { target: { value: '400' } })
    fireEvent.click(screen.getByRole('button', { name: '添削する' }))

    await screen.findByText('添削スコア')
    expect(screen.getByText(/520 \/ 400 文字/)).toBeInTheDocument()
    expect(screen.getByText(/指定字数に収まりませんでした/)).toBeInTheDocument()
  })

  it('上限未指定なら char_limit を送らず、警告も出さない', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      json: async () => ({
        specificity_score: 7, star_score: 6, length_balance_score: 5,
        feedback: 'f', improved_text: 'あ'.repeat(250),
        improved_text_length: 250, char_limit_satisfied: null,
      }),
    })
    setup('ES添削')
    fireEvent.click(screen.getByRole('button', { name: '添削する' }))

    await screen.findByText('添削スコア')
    const body = lastRequestBody()
    expect(body).not.toHaveProperty('char_limit')
    expect(body).not.toHaveProperty('char_limit_mode')
    expect(screen.getByText(/250 文字/)).toBeInTheDocument()
    expect(screen.queryByText(/指定字数に収まりませんでした/)).not.toBeInTheDocument()
  })

  it('範囲外の上限は送信させない（RAGのバリデーション422を踏ませない）', () => {
    setup('ES添削')
    fireEvent.change(screen.getByLabelText(/設問の文字数上限/), { target: { value: '99' } })

    expect(screen.getByRole('button', { name: '添削する' })).toBeDisabled()
    expect(screen.getByText(/100〜2000 の整数で入力してください/)).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText(/設問の文字数上限/), { target: { value: '400' } })
    expect(screen.getByRole('button', { name: '添削する' })).not.toBeDisabled()
  })
})

// 同一設問の前回との差分（#1531）。
// 前回が無い項目に ±0 を出すと「評価された結果の±0」と「比べる相手がいない」が
// 区別できなくなるため、何も出さないことまで固定する。
describe('ESRewritePage 前回との差分 (#1531)', () => {
  const baseResult = {
    specificity_score: 7,
    star_score: 6,
    company_fit_score: null,
    length_balance_score: 5,
    feedback: 'フィードバック本文',
    improved_text: '改善後の文章',
  }

  const reviewWith = async (result: Record<string, unknown>) => {
    ;(global.fetch as jest.Mock).mockResolvedValue({ ok: true, json: async () => result })
    render(<ESRewritePage />)
    fireEvent.click(screen.getByRole('button', { name: 'ES添削' }))
    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: '元の文章です。' },
    })
    fireEvent.click(screen.getByRole('button', { name: '添削する' }))
    await screen.findByText('添削スコア')
  }

  beforeEach(() => {
    global.fetch = jest.fn()
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })


  it('設問を送信本文に載せる', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({ ok: true, json: async () => baseResult })
    render(<ESRewritePage />)
    fireEvent.click(screen.getByRole('button', { name: 'ES添削' }))
    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: '元の文章です。' },
    })
    fireEvent.change(screen.getByPlaceholderText(/学生時代に力を入れたことを教えてください/), {
      target: { value: '学生時代に力を入れたことは？' },
    })
    fireEvent.click(screen.getByRole('button', { name: '添削する' }))
    await screen.findByText('添削スコア')

    const body = JSON.parse((global.fetch as jest.Mock).mock.calls[0][1].body)
    expect(body.question).toBe('学生時代に力を入れたことは？')
  })

  it('前回がある項目は増減を出す', async () => {
    await reviewWith({
      ...baseResult,
      previous: {
        id: 1,
        created_at: '2026-10-01T00:00:00Z',
        question: '設問',
        question_type: 'ガクチカ',
        specificity_score: 4, // 7 - 4 = +3
        star_score: 6, // 変化なし
        length_balance_score: 8, // 5 - 8 = -3
      },
    })

    expect(screen.getByText('▲3')).toBeInTheDocument()
    expect(screen.getByText('±0')).toBeInTheDocument()
    expect(screen.getByText('▼3')).toBeInTheDocument()
    expect(screen.getByText(/同じ設問の前回/)).toBeInTheDocument()
  })

  it('前回が無ければ増減を出さない', async () => {
    await reviewWith(baseResult)

    expect(screen.queryByText('±0')).not.toBeInTheDocument()
    expect(screen.queryByText(/同じ設問の前回/)).not.toBeInTheDocument()
  })
})

describe('ESRewritePage 差分と指摘アンカー (#1532)', () => {
  beforeEach(() => {
    global.fetch = jest.fn()
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })


  it('原文と改善文の追加・削除を示し、指摘から添削時の原文へ移動できる', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      json: async () => ({
        specificity_score: 7,
        star_score: 6,
        company_fit_score: null,
        length_balance_score: 5,
        feedback: '行動を具体的に説明しましょう。',
        improved_text: '私は主体的に取り組みました。',
        feedback_items: [
          {
            score_key: 'specificity_score',
            source_excerpt: '頑張りました',
            issue: '行動の具体性が不足しています。',
            suggestion: 'どのように取り組んだかを書きましょう。',
          },
        ],
      }),
    })

    render(<ESRewritePage />)
    fireEvent.click(screen.getByRole('button', { name: 'ES添削' }))
    const originalField = screen.getByPlaceholderText(/チームで開発した経験があります/)
    fireEvent.change(originalField, { target: { value: '私は頑張りました。' } })
    fireEvent.click(screen.getByRole('button', { name: '添削する' }))

    await screen.findByText('添削スコア')
    expect(document.querySelector('del')).toHaveTextContent('頑張り')
    expect(document.querySelector('ins')).toHaveTextContent('主体的に取り組み')
    expect(document.querySelector('ins')).toHaveStyle({ textDecoration: 'underline' })

    const sourceLink = screen.getByRole('link', { name: '原文の該当箇所を見る' })
    expect(sourceLink).toHaveAttribute('href', '#es-original-anchor-0')
    const anchor = document.getElementById('es-original-anchor-0')
    expect(anchor).toHaveTextContent('頑張り')

    fireEvent.change(originalField, { target: { value: '別の文章に変更しました。' } })
    expect(anchor).toHaveTextContent('頑張り')

    fireEvent.click(sourceLink)
    expect(document.activeElement).toBe(anchor)
  })

  it('差分が簡易表示になる場合はその旨を通知する', async () => {
    const original = '。'.repeat(2100)
    const improved = '、'.repeat(2100)
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      json: async () => ({
        specificity_score: 7,
        star_score: 6,
        company_fit_score: null,
        length_balance_score: 5,
        feedback: 'フィードバック',
        improved_text: improved,
        feedback_items: [],
      }),
    })

    render(<ESRewritePage />)
    fireEvent.click(screen.getByRole('button', { name: 'ES添削' }))
    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: original },
    })
    fireEvent.click(screen.getByRole('button', { name: '添削する' }))

    expect(
      await screen.findByText(/文章が長いため差分を簡易表示しています/),
    ).toBeInTheDocument()
  })

  it('変更がない場合は差分がないことを通知する', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      json: async () => ({
        specificity_score: 7,
        star_score: 6,
        company_fit_score: null,
        length_balance_score: 5,
        feedback: 'フィードバック',
        improved_text: '文章はそのままです。',
        feedback_items: [],
      }),
    })

    render(<ESRewritePage />)
    fireEvent.click(screen.getByRole('button', { name: 'ES添削' }))
    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: '文章はそのままです。' },
    })
    fireEvent.click(screen.getByRole('button', { name: '添削する' }))

    expect(await screen.findByText('改善前後に差分はありませんでした。')).toBeInTheDocument()
  })
})
