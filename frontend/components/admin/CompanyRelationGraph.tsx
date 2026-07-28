'use client'

import { useEffect, useMemo, useState } from 'react'
import ReactFlow, {
  Background,
  Controls,
  MarkerType,
  type Edge,
  type Node,
} from 'reactflow'
import 'reactflow/dist/style.css'
import { Alert, Box, Chip, CircularProgress, Stack, Typography } from '@mui/material'
import { authService } from '@/lib/auth'
import { marketColors, marketLabels, type MarketType } from '@/lib/company-data'
import { layoutCapitalGraph, type CompanyRelationGraph as RelationGraphData } from '@/lib/relation-graph'

interface CompanyRelationGraphProps {
  companyId: number
}

const RELATION_TYPE_LABELS: Record<string, string> = {
  business_partner: '取引先',
  business_procurement: '調達（gBiz）',
  business_subsidy: '補助金（gBiz）',
}

export default function CompanyRelationGraph({ companyId }: CompanyRelationGraphProps) {
  const [graph, setGraph] = useState<RelationGraphData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError('')
    fetch(`/api/admin/companies/${companyId}/relation-graph`, {
      headers: authService.getAdminFetchHeaders(),
    })
      .then((res) => {
        if (!res.ok) throw new Error('相関図データの取得に失敗しました')
        return res.json()
      })
      .then((data: RelationGraphData) => {
        if (!cancelled) setGraph(data)
      })
      .catch(() => {
        if (!cancelled) setError('相関図データの取得に失敗しました')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [companyId])

  const { nodes, edges } = useMemo<{ nodes: Node[]; edges: Edge[] }>(() => {
    if (!graph || graph.nodes.length === 0) return { nodes: [], edges: [] }
    const positions = layoutCapitalGraph(graph)

    const flowNodes: Node[] = graph.nodes.map((n) => {
      const pos = positions.get(n.id) ?? { x: 0, y: 0, level: 0, column: 0 }
      const marketType = (n.market_type || 'unlisted') as MarketType
      return {
        id: String(n.id),
        type: 'default',
        position: { x: pos.x, y: pos.y },
        data: {
          label: (
            <Box sx={{ textAlign: 'center', p: 0.5 }}>
              <Typography variant="body2" sx={{ fontWeight: n.is_focus ? 'bold' : 'normal', mb: 0.5 }}>
                {n.name}
              </Typography>
              <Chip
                size="small"
                label={marketLabels[marketType]}
                sx={{ bgcolor: marketColors[marketType], color: 'white', fontSize: '10px', height: '18px' }}
              />
            </Box>
          ),
        },
        style: {
          background: n.is_focus ? '#FFF3CD' : '#fff',
          border: `${n.is_focus ? 3 : 2}px solid ${n.is_focus ? '#FFC107' : marketColors[marketType]}`,
          borderRadius: '8px',
          padding: '8px',
          minWidth: '180px',
        },
      }
    })

    const flowEdges: Edge[] = graph.capital_edges.map((e, idx) => ({
      id: `capital-${idx}`,
      source: String(e.parent_id),
      target: String(e.child_id),
      label: e.ratio ? `${e.ratio}%` : undefined,
      style: {
        stroke: '#555',
        strokeWidth: 2,
        strokeDasharray: e.relation_type === 'capital_affiliate' ? '5,5' : 'none',
      },
      markerEnd: { type: MarkerType.ArrowClosed, color: '#555' },
    }))

    return { nodes: flowNodes, edges: flowEdges }
  }, [graph])

  if (loading) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, py: 3 }}>
        <CircularProgress size={20} />
        <Typography variant="body2" color="text.secondary">相関図を読み込み中...</Typography>
      </Box>
    )
  }

  if (error) {
    return <Alert severity="error">{error}</Alert>
  }

  if (!graph || graph.nodes.length <= 1) {
    return (
      <Typography variant="body2" color="text.secondary">
        保存済みの資本関係データがありません。関係を確定保存すると相関図が表示されます。
      </Typography>
    )
  }

  return (
    <Stack spacing={1}>
      {graph.truncated && (
        <Alert severity="warning">関係の件数が多いため一部を省略して表示しています。</Alert>
      )}
      <Box sx={{ width: '100%', height: 420, border: '1px solid', borderColor: 'divider', borderRadius: 1 }}>
        <ReactFlow nodes={nodes} edges={edges} fitView minZoom={0.1} maxZoom={2} nodesDraggable nodesConnectable={false}>
          <Background />
          <Controls />
        </ReactFlow>
      </Box>
      {graph.business_relations.length > 0 && (
        <Box>
          <Typography variant="subtitle2" fontWeight="bold" sx={{ mb: 0.5 }}>取引関係</Typography>
          <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
            {graph.business_relations.map((rel) => (
              <Chip
                key={`${rel.company_id}-${rel.relation_type}`}
                size="small"
                label={`${rel.name}（${RELATION_TYPE_LABELS[rel.relation_type] || rel.relation_type}）`}
              />
            ))}
          </Stack>
        </Box>
      )}
    </Stack>
  )
}
