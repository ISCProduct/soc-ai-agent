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
import { sanitizeRelationDescription, displayRelationDescription, isRelationDescriptionFallback } from '@/lib/relation-labels'

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
    if (!graph) return { nodes: [], edges: [] }
    const graphNodes = graph.nodes ?? []
    const capitalEdges = graph.capital_edges ?? []
    if (graphNodes.length === 0) return { nodes: [], edges: [] }

    const positions = layoutCapitalGraph({
      company_id: graph.company_id,
      nodes: graphNodes,
      capital_edges: capitalEdges,
    })

    const flowNodes: Node[] = graphNodes.map((n) => {
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

    const flowEdges: Edge[] = capitalEdges.map((e, idx) => ({
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

  const graphNodes = graph?.nodes ?? []
  const businessRelations = graph?.business_relations ?? []
  const hasCapitalGraph = graphNodes.length > 1
  const hasBusinessRelations = businessRelations.length > 0

  if (!graph || (!hasCapitalGraph && !hasBusinessRelations)) {
    return (
      <Typography variant="body2" color="text.secondary">
        保存済みの関係データがありません。関係を確定保存すると相関図・取引関係が表示されます。
      </Typography>
    )
  }

  return (
    <Stack spacing={1}>
      {graph.truncated && (
        <Alert severity="warning">関係の件数が多いため一部を省略して表示しています。</Alert>
      )}
      {hasCapitalGraph && (
        <Box
          sx={{
            width: '100%',
            height: { xs: 480, sm: 560, md: 'min(72vh, 720px)' },
            minHeight: 420,
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 1,
            bgcolor: 'grey.50',
          }}
        >
          <ReactFlow nodes={nodes} edges={edges} fitView minZoom={0.1} maxZoom={2} nodesDraggable nodesConnectable={false}>
            <Background />
            <Controls />
          </ReactFlow>
        </Box>
      )}
      {!hasCapitalGraph && hasBusinessRelations && (
        <Typography variant="body2" color="text.secondary">
          資本関係データはありません。取引関係のみ表示しています。
        </Typography>
      )}
      {hasBusinessRelations && (
        <Box>
          <Typography variant="subtitle2" fontWeight="bold" sx={{ mb: 1 }}>
            取引関係
          </Typography>
          <Stack spacing={1}>
            {businessRelations.map((rel) => {
              const detail = displayRelationDescription(rel.description || '', rel.relation_type)
              const isFallback = isRelationDescriptionFallback(rel.description || '')
              const typeLabel = RELATION_TYPE_LABELS[rel.relation_type] || rel.relation_type
              return (
                <Box
                  key={`${rel.company_id}-${rel.relation_type}-${rel.name}`}
                  sx={{
                    px: 1.5,
                    py: 1,
                    border: '1px solid',
                    borderColor: 'divider',
                    borderRadius: '8px',
                    bgcolor: 'background.paper',
                  }}
                >
                  <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
                    <Typography variant="body2" fontWeight={700}>
                      {rel.name}
                    </Typography>
                    <Chip size="small" label={typeLabel} />
                    {isFallback && <Chip size="small" variant="outlined" label="内容未特定" />}
                  </Stack>
                  <Typography variant="body2" color={isFallback ? 'text.secondary' : 'text.primary'} sx={{ mt: 0.5 }}>
                    {detail}
                  </Typography>
                </Box>
              )
            })}
          </Stack>
        </Box>
      )}
    </Stack>
  )
}
