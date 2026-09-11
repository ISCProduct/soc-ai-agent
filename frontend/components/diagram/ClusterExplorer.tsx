'use client'

import { useMemo, useState } from 'react'
import { Box, Typography, TextField, Chip } from '@mui/material'
import { marketColors, marketLabels, type CapitalRelation, type MarketType } from '@/lib/company-data'
import { computeRelationClusters, type RelationCluster } from '@/lib/relation-graph'

type DiagramType = 'capital' | 'business'

interface ClusterExplorerProps {
  relations: CapitalRelation[]
  diagramType: DiagramType
  getCompanyName: (companyId: number) => string
  getMarketType: (companyId: number) => MarketType
  onSelectCompany: (companyId: number) => void
}

/**
 * 起点企業を指定しない「全体表示」の代わりに使うクラスタカード一覧。
 * #970フォローアップ: 数百社をまとめて1枚のReactFlowグラフに描画すると、ハブ企業
 * (メガバンク等)が複数クラスタにまたがる関係を持つためエッジが画面を横断し、
 * かつノード数過多でfitViewが縮小しすぎて判読不能になる。
 * 「1枚のグラフに全部描く」のをやめ、資本/事業関係で繋がっている企業群(連結成分)を
 * カード単位でまとめて提示し、選ぶとその代表企業を起点にした単一企業表示(既に読みやすい)
 * へ遷移する設計にした。
 */
export default function ClusterExplorer({
  relations,
  diagramType,
  getCompanyName,
  getMarketType,
  onSelectCompany,
}: ClusterExplorerProps) {
  const [query, setQuery] = useState('')

  const clusters = useMemo(() => {
    const companyIds = new Set<number>()
    relations.forEach((rel) => {
      if (diagramType === 'capital') {
        if (rel.parent_id) companyIds.add(rel.parent_id)
        if (rel.child_id) companyIds.add(rel.child_id)
      } else {
        if (rel.from_id) companyIds.add(rel.from_id)
        if (rel.to_id) companyIds.add(rel.to_id)
      }
    })
    return computeRelationClusters(Array.from(companyIds), relations)
  }, [relations, diagramType])

  const filteredClusters = useMemo(() => {
    const q = query.trim()
    if (!q) return clusters
    return clusters.filter((cluster) =>
      cluster.memberIds.some((id) => getCompanyName(id).includes(q)),
    )
  }, [clusters, query, getCompanyName])

  return (
    <Box sx={{ height: '100%', overflowY: 'auto', p: 2, bgcolor: '#fafafa' }}>
      <Box sx={{ maxWidth: 720, mx: 'auto' }}>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          {diagramType === 'capital' ? '資本関係' : '事業関係'}で繋がっている企業群を{filteredClusters.length}件表示中
          （全{clusters.reduce((sum, c) => sum + c.memberIds.length, 0)}社）。
          カードを選ぶとその企業群を起点企業表示で詳しく見られます。
        </Typography>
        <TextField
          size="small"
          fullWidth
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="企業名で絞り込み"
          sx={{ mb: 2, bgcolor: '#fff' }}
        />
        {filteredClusters.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            該当する企業群が見つかりませんでした。
          </Typography>
        ) : (
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
            {filteredClusters.map((cluster) => (
              <ClusterCard
                key={cluster.hubId}
                cluster={cluster}
                getCompanyName={getCompanyName}
                getMarketType={getMarketType}
                onSelect={() => onSelectCompany(cluster.hubId)}
              />
            ))}
          </Box>
        )}
      </Box>
    </Box>
  )
}

function ClusterCard({
  cluster,
  getCompanyName,
  getMarketType,
  onSelect,
}: {
  cluster: RelationCluster
  getCompanyName: (companyId: number) => string
  getMarketType: (companyId: number) => MarketType
  onSelect: () => void
}) {
  const hubName = getCompanyName(cluster.hubId)
  const otherMembers = cluster.memberIds.filter((id) => id !== cluster.hubId)
  const previewNames = otherMembers.slice(0, 4).map((id) => getCompanyName(id))
  const remaining = otherMembers.length - previewNames.length
  const marketType = getMarketType(cluster.hubId)

  return (
    <Box
      onClick={onSelect}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') onSelect()
      }}
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 2,
        p: 2,
        bgcolor: '#fff',
        border: '1px solid #ddd',
        borderRadius: '8px',
        cursor: 'pointer',
        '&:hover': { borderColor: marketColors[marketType], boxShadow: '0 2px 8px rgba(0,0,0,0.08)' },
      }}
    >
      <Chip
        label={cluster.memberIds.length === 1 ? '単独' : `${cluster.memberIds.length}社`}
        size="small"
        sx={{ bgcolor: marketColors[marketType], color: '#fff', fontWeight: 600, flexShrink: 0 }}
      />
      <Box sx={{ minWidth: 0, flex: 1 }}>
        <Typography variant="body1" fontWeight={600} noWrap>
          {hubName}
        </Typography>
        {previewNames.length > 0 && (
          <Typography variant="caption" color="text.secondary" noWrap component="div">
            関連: {previewNames.join('、')}
            {remaining > 0 ? ` 他${remaining}社` : ''}
          </Typography>
        )}
      </Box>
      <Chip label={marketLabels[marketType]} size="small" variant="outlined" sx={{ flexShrink: 0 }} />
    </Box>
  )
}
