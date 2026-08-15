'use client'

import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { InterviewTrendPoint } from '@/lib/interview'
import { PRIMARY } from '../constants'

type InterviewTrendChartProps = {
  points: InterviewTrendPoint[]
}

/** recharts 依存を履歴ページ本体から切り離すためのチャート表示。 */
export default function InterviewTrendChart({ points }: InterviewTrendChartProps) {
  const data = points.map((p, i) => ({
    ...p,
    label: `#${p.session_id}`,
    index: i + 1,
  }))

  return (
    <ResponsiveContainer width="100%" height="100%">
      <LineChart data={data}>
        <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
        <XAxis dataKey="label" tick={{ fontSize: 12, fill: '#64748b' }} />
        <YAxis domain={[0, 10]} tick={{ fontSize: 12, fill: '#64748b' }} width={28} />
        <Tooltip formatter={(v: number) => v?.toFixed(1)} />
        <Legend wrapperStyle={{ fontSize: 12 }} />
        <Line type="monotone" dataKey="logic" name="論理性" stroke={PRIMARY} dot={{ r: 3 }} activeDot={{ r: 5 }} connectNulls />
        <Line type="monotone" dataKey="specificity" name="具体性" stroke="#3b82f6" dot={{ r: 3 }} activeDot={{ r: 5 }} connectNulls />
        <Line type="monotone" dataKey="ownership" name="主体性" stroke="#10b981" dot={{ r: 3 }} activeDot={{ r: 5 }} connectNulls />
        <Line type="monotone" dataKey="communication" name="コミュニケーション" stroke="#8b5cf6" dot={{ r: 3 }} activeDot={{ r: 5 }} connectNulls />
        <Line type="monotone" dataKey="enthusiasm" name="熱意" stroke="#f59e0b" dot={{ r: 3 }} activeDot={{ r: 5 }} connectNulls />
      </LineChart>
    </ResponsiveContainer>
  )
}
