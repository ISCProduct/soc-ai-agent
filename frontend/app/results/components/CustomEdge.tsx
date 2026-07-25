'use client'

import type { EdgeProps, EdgeTypes } from 'reactflow'

/**
 * ReactFlow のカスタムエッジ（直線 + 回転ラベル）。
 */
export const CustomEdge = ({ id, sourceX, sourceY, targetX, targetY, style, markerEnd, label }: EdgeProps) => {
  const edgePath = `M ${sourceX} ${sourceY} L ${targetX} ${targetY}`

  // ラベルの位置を計算（中点）
  const labelX = (sourceX + targetX) / 2
  const labelY = (sourceY + targetY) / 2

  // エッジの角度を計算
  const angle = Math.atan2(targetY - sourceY, targetX - sourceX) * (180 / Math.PI)

  // テキストが逆さまにならないように調整（-90度〜90度の範囲に収める）
  const adjustedAngle = angle > 90 || angle < -90 ? angle + 180 : angle

  return (
    <>
      <path
        id={id}
        style={style}
        className="react-flow__edge-path"
        d={edgePath}
        markerEnd={markerEnd}
      />
      {label && (
        <text
          x={labelX}
          y={labelY}
          style={{
            fontSize: '13px',
            fill: '#333',
            fontWeight: 600,
            pointerEvents: 'none',
          }}
          textAnchor="middle"
          dominantBaseline="middle"
          transform={`rotate(${adjustedAngle}, ${labelX}, ${labelY})`}
        >
          {/* 白い縁取り（背景） */}
          <tspan
            x={labelX}
            dy="0"
            style={{
              fill: 'none',
              stroke: '#fff',
              strokeWidth: 4,
              strokeLinejoin: 'round',
              paintOrder: 'stroke',
            }}
          >
            {label}
          </tspan>
          {/* メインテキスト */}
          <tspan
            x={labelX}
            dy="0"
            style={{
              fill: '#333',
            }}
          >
            {label}
          </tspan>
        </text>
      )}
    </>
  )
}

export const edgeTypes: EdgeTypes = {
  custom: CustomEdge,
}
