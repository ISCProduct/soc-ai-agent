'use client'

import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

export default function CostBarChart({
  data,
  valueKey,
  labelKey,
}: {
  data: Record<string, unknown>[]
  valueKey: string
  labelKey: string
}) {
  if (data.length === 0) {
    return <p className="text-center text-sm text-muted-foreground py-8">データなし</p>
  }
  return (
    <div className="h-56 mt-2">
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" className="stroke-border" vertical={false} />
          <XAxis
            dataKey={labelKey}
            tick={{ fontSize: 11 }}
            tickLine={false}
            axisLine={{ className: 'stroke-border' } as never}
          />
          <YAxis
            tick={{ fontSize: 11 }}
            tickLine={false}
            axisLine={false}
            width={64}
            tickFormatter={(v: number) => `$${v.toFixed(2)}`}
          />
          <Tooltip
            formatter={(value: number) => [`$${value.toFixed(4)}`, 'コスト']}
            contentStyle={{ fontSize: 12, borderRadius: 8 }}
          />
          <Bar dataKey={valueKey} fill="var(--chart-1)" radius={[3, 3, 0, 0]} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}
