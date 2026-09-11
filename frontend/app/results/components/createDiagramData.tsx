'use client'

import { Box, Typography, Chip } from '@mui/material'
import {
  Node,
  Edge,
  MarkerType,
} from 'reactflow'
import {
  marketColors,
  marketLabels,
  type CapitalRelation,
  type CompanyMarketInfo,
  type MarketType,
} from '@/lib/company-data'
import { formatRelationLabel } from '@/lib/relation-labels'
import { layoutBusinessGraph, layoutCapitalGraphFromEdges } from '@/lib/relation-graph'
import { collectRelatedCompanyIds, resolveCompanyNameFromRelations } from '../utils'

/**
 * 資本 / ビジネス関連図のノード・エッジを生成する。
 * ノードラベルに JSX を含むため .tsx に配置する。
 */
export function createDiagramData(
  companyId: string,
  type: 'capital' | 'business',
  relations: CapitalRelation[],
  marketInfo: CompanyMarketInfo[],
): { nodes: Node[]; edges: Edge[] } {
  const compId = Number.parseInt(companyId, 10)
  if (Number.isNaN(compId)) {
    return { nodes: [], edges: [] }
  }
  const relatedIds = collectRelatedCompanyIds(compId, type, relations)

  const getMarketType = (id: number): MarketType => {
    const info = marketInfo.find((m) => m.company_id === id)
    return info?.market_type || 'unlisted'
  }

  // ノード生成
  // #970: 登録順の円形配置ではなく、起点企業からの関係性(BFS距離)に基づいて配置する
  const nodes: Node[] = []
  const ids = Array.from(relatedIds)
  const positions =
    type === 'capital'
      ? layoutCapitalGraphFromEdges(compId, ids, relations)
      : layoutBusinessGraph(compId, ids, relations)

  ids.forEach((id) => {
    const isFocusCompany = id === compId
    const marketType = getMarketType(id)
    const pos = positions.get(id) ?? { x: 0, y: 0 }

    nodes.push({
      id: String(id),
      type: 'default',
      position: { x: pos.x, y: pos.y },
      data: {
        label: (
          <Box sx={{ textAlign: 'center', p: 1 }}>
            <Typography
              variant="body2"
              sx={{ fontWeight: isFocusCompany ? 'bold' : 'normal', mb: 0.5 }}
            >
              {resolveCompanyNameFromRelations(id, relations)}
            </Typography>
            <Chip
              label={marketLabels[marketType]}
              size="small"
              sx={{
                bgcolor: marketColors[marketType],
                color: 'white',
                fontSize: '10px',
                height: '20px',
              }}
            />
          </Box>
        ),
      },
      style: {
        background: isFocusCompany ? '#FFF3CD' : '#fff',
        border: `3px solid ${isFocusCompany ? '#FFC107' : marketColors[marketType]}`,
        borderRadius: '8px',
        padding: '10px',
        minWidth: '200px',
        boxShadow: isFocusCompany ? '0 4px 12px rgba(255, 193, 7, 0.3)' : undefined,
      },
    })
  })

  // エッジ生成
  const edges: Edge[] = []
  relations.forEach((rel, idx) => {
    if (type === 'capital' && rel.relation_type.startsWith('capital') && rel.parent_id && rel.child_id) {
      if (relatedIds.has(rel.parent_id) && relatedIds.has(rel.child_id)) {
        edges.push({
          id: `capital-${idx}`,
          source: String(rel.parent_id),
          target: String(rel.child_id),
          type: 'custom',
          label: rel.ratio ? `${rel.ratio.toFixed(0)}%` : '',
          style: {
            stroke: '#555',
            strokeWidth: 2,
            strokeDasharray: rel.relation_type === 'capital_affiliate' ? '5,5' : 'none',
          },
          markerEnd: {
            type: MarkerType.ArrowClosed,
            color: '#555',
          },
        })
      }
    } else if (type === 'business' && rel.relation_type.startsWith('business') && rel.from_id && rel.to_id) {
      if (relatedIds.has(rel.from_id) && relatedIds.has(rel.to_id)) {
        edges.push({
          id: `business-${idx}`,
          source: String(rel.from_id),
          target: String(rel.to_id),
          type: 'custom',
          label: formatRelationLabel(rel.description, rel.relation_type),
          animated: true,
          style: {
            stroke: '#2196F3',
            strokeWidth: 2,
          },
          markerEnd: {
            type: MarkerType.ArrowClosed,
            color: '#2196F3',
          },
        })
      }
    }
  })

  return { nodes, edges }
}
