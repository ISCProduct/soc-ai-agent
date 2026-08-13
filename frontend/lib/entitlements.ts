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

export function canUseFeature(ent: Entitlements | null, feature: FeatureKey): boolean {
  if (!ent) return true
  return ent.features[feature] !== false
}

export async function fetchEntitlements(): Promise<Entitlements> {
  const res = await fetch('/api/entitlements', { cache: 'no-store' })
  if (!res.ok) throw new Error('entitlements')
  return res.json() as Promise<Entitlements>
}
