'use client'

import type { EdgeProps, EdgeTypes } from 'reactflow'

/**
 * 関連図用エッジ。ラベルは水平のピル型で重なりを抑える。
 */
export function RelationEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  style,
  markerEnd,
  label,
}: EdgeProps) {
  const edgePath = `M ${sourceX} ${sourceY} L ${targetX} ${targetY}`
  const labelX = (sourceX + targetX) / 2
  const labelY = (sourceY + targetY) / 2
  const text = typeof label === 'string' ? label : label != null ? String(label) : ''
  const visualWidth = Array.from(text).reduce((sum, ch) => sum + (/[\u3000-\u30ff\u3400-\u9fff\uff00-\uffef]/.test(ch) ? 2 : 1), 0)
  const maxVisualWidth = 16
  const truncatedText = (() => {
    if (visualWidth <= maxVisualWidth) return text
    let used = 0
    let out = ''
    for (const ch of Array.from(text)) {
      const width = /[\u3000-\u30ff\u3400-\u9fff\uff00-\uffef]/.test(ch) ? 2 : 1
      if (used+ width > maxVisualWidth - 1) break
      used += width
      out += ch
    }
    return `${out}…`
  })()
  const truncatedWidth = Array.from(truncatedText).reduce((sum, ch) => sum + (/[\u3000-\u30ff\u3400-\u9fff\uff00-\uffef]/.test(ch) ? 2 : 1), 0)
  const pillWidth = Math.min(140, Math.max(36, truncatedWidth * 7.2 + 18))

  return (
    <>
      <path
        id={id}
        style={style}
        className="react-flow__edge-path"
        d={edgePath}
        markerEnd={markerEnd}
      />
      {text ? (
        <g transform={`translate(${labelX}, ${labelY})`} style={{ pointerEvents: 'none' }}>
          <rect
            x={-pillWidth / 2}
            y={-11}
            width={pillWidth}
            height={22}
            rx={11}
            fill="#ffffff"
            stroke="#d0d5dd"
            strokeWidth={1}
          />
          <text
            textAnchor="middle"
            dominantBaseline="central"
            style={{
              fontSize: 11,
              fontWeight: 600,
              fill: '#344054',
              letterSpacing: '0.01em',
            }}
          >
            {truncatedText}
          </text>
        </g>
      ) : null}
    </>
  )
}

export const relationEdgeTypes: EdgeTypes = {
  custom: RelationEdge,
}

/** @deprecated results 側の互換 export */
export const CustomEdge = RelationEdge
export const edgeTypes = relationEdgeTypes
