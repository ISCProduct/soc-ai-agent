export function correlationLabel(r: number): string {
  const abs = Math.abs(r)
  if (abs >= 0.7) return '強い相関'
  if (abs >= 0.4) return '中程度の相関'
  if (abs >= 0.2) return '弱い相関'
  return 'ほぼ無相関'
}

export function correlationColor(r: number): string {
  const abs = Math.abs(r)
  if (abs >= 0.7) return '#1976d2'
  if (abs >= 0.4) return '#388e3c'
  if (abs >= 0.2) return '#f57c00'
  return '#9e9e9e'
}

export function formatPercent(v: number): string {
  return `${(v * 100).toFixed(1)}%`
}
