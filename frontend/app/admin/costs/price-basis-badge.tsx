import { Badge } from '@/components/ui/badge'

/**
 * 単価の根拠（#2004）。バックエンドの costs.PriceBasis* と同じ値。
 * - listed: 公開価格（または運用で上書きした単価）
 * - estimated: 実価格を確認していない推定値
 * - unknown: 単価表に無い。少なく見積もらないよう gpt-4o の単価で計算している
 */
export type PriceBasis = 'listed' | 'estimated' | 'unknown'

const LABELS: Record<Exclude<PriceBasis, 'listed'>, { label: string; title: string }> = {
  estimated: {
    label: '推定単価',
    title: '実価格を確認していない推定の単価で計算しています。請求額とずれる可能性があります。',
  },
  unknown: {
    label: '単価未登録',
    title: '単価表に無いモデルのため、gpt-4o の単価で計算しています。請求額とずれる可能性があります。',
  },
}

/** 推定・未登録の単価で計算した金額に印を付ける。公開価格（listed）と、単価を使っていない行（空）は何も出さない。 */
export function PriceBasisBadge({ basis }: { basis?: string }) {
  if (basis !== 'estimated' && basis !== 'unknown') return null
  const { label, title } = LABELS[basis]
  return (
    <Badge variant="secondary" title={title} aria-label={`${label}：${title}`}>
      {label}
    </Badge>
  )
}
