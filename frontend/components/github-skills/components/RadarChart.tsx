import { AXES_ORDER, CATEGORY_COLORS, CATEGORY_LABELS } from '../constants'
import {
  buildRadarAxisLabels,
  buildRadarDataPolygon,
  buildRadarGridPolygons,
  radarPointOnAxis,
} from '../utils'
import type { RadarChartProps } from '../types'

export function RadarChart({ scores, size = 240 }: RadarChartProps) {
  const center = size / 2
  const radius = size * 0.38
  const n = AXES_ORDER.length
  const scoreMap = Object.fromEntries(scores.map(s => [s.Category, s.Score]))

  const gridPolygons = buildRadarGridPolygons(center, radius, n)
  const dataPoints = buildRadarDataPolygon(scores, center, radius)
  const labels = buildRadarAxisLabels(center, radius, 22, CATEGORY_LABELS, CATEGORY_COLORS)

  return (
    <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
      {gridPolygons.map((pts, i) => (
        <polygon
          key={i}
          points={pts}
          fill="none"
          stroke="#334155"
          strokeWidth="1"
          strokeDasharray={i < gridPolygons.length - 1 ? '3,3' : undefined}
        />
      ))}
      {AXES_ORDER.map((_, i) => {
        const outer = radarPointOnAxis(i, 1, center, radius, n)
        return (
          <line
            key={i}
            x1={center}
            y1={center}
            x2={outer.x}
            y2={outer.y}
            stroke="#334155"
            strokeWidth="1"
          />
        )
      })}
      <polygon
        points={dataPoints}
        fill="rgba(79,195,247,0.25)"
        stroke="#4FC3F7"
        strokeWidth="2"
      />
      {AXES_ORDER.map((cat, i) => {
        const score = scoreMap[cat] ?? 0
        const p = radarPointOnAxis(i, score / 100, center, radius, n)
        return (
          <circle
            key={i}
            cx={p.x}
            cy={p.y}
            r="4"
            fill={CATEGORY_COLORS[cat] ?? '#4FC3F7'}
          />
        )
      })}
      {labels.map((l, i) => (
        <text
          key={i}
          x={l.x}
          y={l.y}
          textAnchor="middle"
          dominantBaseline="middle"
          fontSize="10"
          fill={l.color}
          fontWeight="600"
        >
          {l.label}
        </text>
      ))}
    </svg>
  )
}
