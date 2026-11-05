'use client'

import { useId, useState } from 'react'
import type { Edge } from 'reactflow'
import { Box, Button, Typography } from '@mui/material'

/** 相関図の辺に載せる意味。文章での言い換えに使う（#1614）。 */
export type RelationEdgeData = { kind: string }

/** 辺1本を「A → B：子会社（60%）」の1文にする。 */
export function describeRelationEdge(edge: Edge<RelationEdgeData>, getCompanyName: (id: number) => string): string {
  const label = typeof edge.label === 'string' && edge.label ? `（${edge.label}）` : ''
  return `${getCompanyName(Number(edge.source))} → ${getCompanyName(Number(edge.target))}：${edge.data?.kind ?? '関係'}${label}`
}

type Props = {
  edges: Edge<RelationEdgeData>[]
  companyCount: number
  getCompanyName: (id: number) => string
  /** 渡すと各行に「〇〇の情報」ボタンを出す。キーボードで企業を辿る経路を兼ねる。 */
  onSelectCompany?: (companyId: number) => void
  focusCompanyId?: number | null
}

/**
 * 相関図と同じ内容の文章による代替表現（#1614）。
 * 図は視覚とマウス前提なので、読み上げ・キーボードの利用者にも同じ情報を別経路で渡す。
 */
export function RelationTextList({ edges, companyCount, getCompanyName, onSelectCompany, focusCompanyId }: Props) {
  const [open, setOpen] = useState(false)
  const listId = useId()
  const summary = `${companyCount}社・${edges.length}件の関係`

  return (
    <Box sx={{ borderTop: 1, borderColor: 'divider', bgcolor: 'background.paper', flexShrink: 0 }}>
      <Button
        size="small"
        aria-expanded={open}
        aria-controls={listId}
        onClick={() => setOpen((v) => !v)}
        sx={{ m: 0.5 }}
      >
        {open ? '関係の一覧を閉じる' : `関係を一覧で見る（${summary}）`}
      </Button>
      {open && (
        <Box
          id={listId}
          component="section"
          aria-label={`関係の一覧（${summary}）`}
          sx={{ maxHeight: 220, overflowY: 'auto', px: 2, pb: 1.5 }}
        >
          {edges.length === 0 ? (
            <Typography variant="body2" color="text.secondary">この企業を起点とした関係は登録されていません。</Typography>
          ) : (
            <Box component="ul" sx={{ m: 0, pl: 2.5 }}>
              {edges.map((e) => {
                const other = Number(e.source) === focusCompanyId ? Number(e.target) : Number(e.source)
                return (
                  <Box component="li" key={e.id} sx={{ py: 0.25 }}>
                    <Typography variant="body2" component="span">{describeRelationEdge(e, getCompanyName)}</Typography>
                    {onSelectCompany && (
                      <Button size="small" sx={{ ml: 1, py: 0 }} onClick={() => onSelectCompany(other)}>
                        {`${getCompanyName(other)}の情報`}
                      </Button>
                    )}
                  </Box>
                )
              })}
            </Box>
          )}
        </Box>
      )}
    </Box>
  )
}
