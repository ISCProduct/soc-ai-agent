'use client'

import { useEffect, useMemo } from 'react'
import {
  Box,
  Typography,
  CircularProgress,
} from '@mui/material'
import ReactFlow, {
  Node,
  Edge,
  Controls,
  Background,
  MiniMap,
  useNodesState,
  useEdgesState,
} from 'reactflow'
import 'reactflow/dist/style.css'
import {
  marketColors,
  marketLabels,
  type CapitalRelation,
  type CompanyMarketInfo,
  type MarketType,
} from '@/lib/company-data'
import { edgeTypes } from './CustomEdge'
import { createDiagramData } from './createDiagramData'

export interface CompanyRelationDiagramProps {
  companyId: string
  type: 'capital' | 'business'
  relations: CapitalRelation[]
  marketInfo: CompanyMarketInfo[]
  loading: boolean
  error: string | null
}

/**
 * 資本 / ビジネス関連図の共通 ReactFlow ビュー。
 */
export default function CompanyRelationDiagram({
  companyId,
  type,
  relations,
  marketInfo,
  loading,
  error,
}: CompanyRelationDiagramProps) {
  const diagram = useMemo(
    () => createDiagramData(companyId, type, relations, marketInfo),
    [companyId, type, relations, marketInfo],
  )

  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])

  useEffect(() => {
    setNodes(diagram.nodes)
    setEdges(diagram.edges)
  }, [diagram, setNodes, setEdges])

  if (loading) {
    return (
      <Box sx={{ height: { xs: 320, sm: 450, md: 600 } }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
          <CircularProgress />
        </Box>
      </Box>
    )
  }

  if (error) {
    return (
      <Box sx={{ height: { xs: 320, sm: 450, md: 600 } }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
          <Typography color="error">{error}</Typography>
        </Box>
      </Box>
    )
  }

  if (diagram.nodes.length === 0) {
    return (
      <Box sx={{ height: { xs: 320, sm: 450, md: 600 } }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
          <Typography color="text.secondary">
            {type === 'capital'
              ? 'この企業の資本関連情報はありません'
              : 'この企業のビジネス関連情報はありません'}
          </Typography>
        </Box>
      </Box>
    )
  }

  return (
    <Box sx={{ height: { xs: 320, sm: 450, md: 600 } }}>
      {type === 'capital' ? (
        <Box sx={{ mb: 2, display: 'flex', gap: 3, fontSize: '12px', flexWrap: 'wrap' }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
            <Box sx={{ width: 40, height: 2, bgcolor: '#555' }} />
            <span>子会社（実線）</span>
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
            <Box sx={{ width: 40, height: 2, borderTop: '2px dashed #555' }} />
            <span>関連会社（破線）</span>
          </Box>
          {Object.entries(marketLabels).map(([key, label]) => (
            <Box key={key} sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              <Box sx={{ width: 16, height: 16, bgcolor: marketColors[key as MarketType], borderRadius: '50%' }} />
              <Typography variant="caption">{label}</Typography>
            </Box>
          ))}
        </Box>
      ) : (
        <Box sx={{ mb: 2, display: 'flex', gap: 2, flexWrap: 'wrap' }}>
          {Object.entries(marketLabels).map(([key, label]) => (
            <Box key={key} sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              <Box sx={{ width: 16, height: 16, bgcolor: marketColors[key as MarketType], borderRadius: '50%' }} />
              <Typography variant="caption">{label}</Typography>
            </Box>
          ))}
        </Box>
      )}
      <Box sx={{ height: 'calc(100% - 40px)', border: '1px solid #e0e0e0', borderRadius: 1 }}>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          edgeTypes={edgeTypes}
          fitView
          minZoom={0.05}
          maxZoom={3}
          defaultViewport={{ x: 0, y: 0, zoom: 0.8 }}
          attributionPosition="bottom-right"
          nodesDraggable={true}
          nodesConnectable={false}
          elementsSelectable={true}
        >
          <Background color="#aaa" gap={16} />
          <Controls
            showZoom={true}
            showFitView={true}
            showInteractive={true}
            position="top-right"
          />
          <MiniMap
            nodeColor={(node) => {
              const border = node.style?.border as string
              if (border?.includes('#FFA726')) return '#FFA726'
              return '#2196F3'
            }}
            maskColor="rgba(0, 0, 0, 0.1)"
            position="bottom-left"
          />
        </ReactFlow>
      </Box>
    </Box>
  )
}
