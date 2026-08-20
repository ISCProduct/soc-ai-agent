export type PlanID = 'free' | 'standard' | 'pro'

export type FeatureKey =
  | 'matching'
  | 'interview'
  | 'resume'
  | 'company_graph'
  | 'admin'
  | 'export'

export type Entitlements = {
  plan: PlanID
  features: Partial<Record<FeatureKey, boolean>>
}

// #989: entitlements未取得/取得失敗時に機能を許可するfail-open実装は、
// バックエンドが独立して同じ制限を課すため実データ漏洩には至らないものの、
// UI上は本来無効な機能が一時的に有効表示されてしまっていた。fail-closedにし、
// 明示的にtrueが返る場合のみ許可する。
export function canUseFeature(ent: Entitlements | null, feature: FeatureKey): boolean {
  if (!ent) return false
  return ent.features[feature] === true
}

export async function fetchEntitlements(): Promise<Entitlements> {
  const res = await fetch('/api/entitlements', { cache: 'no-store' })
  if (!res.ok) throw new Error('entitlements')
  return res.json() as Promise<Entitlements>
}
