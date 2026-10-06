import {
  ADMIN_COLORS,
  FRESHNESS,
  resolveFreshness,
  TTL_DAYS,
  type Freshness,
} from '@/lib/design-tokens'
import { COMFORTABLE_PRIMARY } from '@/lib/student-theme'

const NOW = new Date('2026-10-01T00:00:00Z')

function daysAgo(n: number): Date {
  return new Date(NOW.getTime() - n * 86_400_000)
}

describe('resolveFreshness', () => {
  // TTL 90日・dueRatio 0.2 なので、72日で「期限が近い」、90日で「期限切れ」。
  const cases: Array<{ name: string; at: Date | string | null | undefined; want: Freshness }> = [
    { name: '取得日時が無ければ未取得', at: null, want: 'unfetched' },
    { name: 'undefined も未取得', at: undefined, want: 'unfetched' },
    { name: '空文字は未取得（不正な日付を期限切れに倒さない）', at: '', want: 'unfetched' },
    { name: '不正な日付文字列は未取得', at: 'not-a-date', want: 'unfetched' },
    { name: '取得直後は取得済み', at: daysAgo(0), want: 'fresh' },
    { name: '71日は取得済み（しきい値の直前）', at: daysAgo(71), want: 'fresh' },
    { name: '72日で期限が近い（しきい値ちょうど）', at: daysAgo(72), want: 'due' },
    { name: '89日はまだ期限が近い', at: daysAgo(89), want: 'due' },
    { name: '90日で期限切れ（TTLちょうど）', at: daysAgo(90), want: 'expired' },
    { name: '120日は期限切れ', at: daysAgo(120), want: 'expired' },
  ]

  it.each(cases)('$name', ({ at, want }) => {
    expect(resolveFreshness(at, TTL_DAYS.info, NOW)).toBe(want)
  })

  it('TTLが短い系統でも同じ境界で判定する（jobs 7日）', () => {
    expect(resolveFreshness(daysAgo(5), TTL_DAYS.jobs, NOW)).toBe('fresh')
    expect(resolveFreshness(daysAgo(6), TTL_DAYS.jobs, NOW)).toBe('due')
    expect(resolveFreshness(daysAgo(7), TTL_DAYS.jobs, NOW)).toBe('expired')
  })

  it('Date と ISO文字列で同じ結果になる', () => {
    const at = daysAgo(100)
    expect(resolveFreshness(at, TTL_DAYS.info, NOW)).toBe(
      resolveFreshness(at.toISOString(), TTL_DAYS.info, NOW),
    )
  })
})

describe('FRESHNESS', () => {
  it('取得済み以外は字形を持つ（色だけで状態を伝えない）', () => {
    // §19: 色だけで状態を表さない。対応が必要な3状態は字形で区別できること。
    expect(FRESHNESS.due.mark).not.toBe('')
    expect(FRESHNESS.expired.mark).not.toBe('')
    expect(FRESHNESS.unfetched.mark).not.toBe('')
  })

  it('字形が互いに重複しない', () => {
    const marks = (['due', 'expired', 'unfetched'] as const).map((k) => FRESHNESS[k].mark)
    expect(new Set(marks).size).toBe(marks.length)
  })

  it('すべての状態に日本語ラベルがある（読み上げ用）', () => {
    for (const key of Object.keys(FRESHNESS) as Freshness[]) {
      expect(FRESHNESS[key].label).not.toBe('')
    }
  })
})

describe('ADMIN_COLORS', () => {
  it('学生側の primary と MUI 既定の両方と別の色を使う', () => {
    // 管理画面に独立した identity を与える判断（2026-10-01）。
    // 取り違えると「別系統にした」つもりが既定色のままになる。
    expect(ADMIN_COLORS.indigo).not.toBe(COMFORTABLE_PRIMARY)
    expect(ADMIN_COLORS.indigo).not.toBe('#1976d2')
  })

  it('本文色を #111 系の黒代わりにしない', () => {
    // AI生成UIの典型を避ける意図をテストで固定する。
    expect(['#111', '#111111', '#0b0b0b', '#000000']).not.toContain(ADMIN_COLORS.ink.toLowerCase())
  })
})
