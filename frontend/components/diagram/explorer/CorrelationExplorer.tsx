'use client'

import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Box, Button, Chip, SwipeableDrawer, Tab, Tabs, useMediaQuery, useTheme } from '@mui/material'
import { Alert } from '@/components/common/Alert'
import { PageLoading } from '@/components/common/PageLoading'
import { childrenOf, partnersOf, companyTags, topBusinessTags, fetchCorrelationMap, sortCompanies, type CorrelationMap, type CorrelationMode } from '@/lib/company/correlation-map'
import { CompanyGraph } from './CompanyGraph'
import { CompanyDetails } from './CompanyDetails'
import { ExpandedGraph } from './ExpandedGraph'
import { revealCompany } from './graph-viewport'
import styles from './explorer.module.css'

export default function CorrelationExplorer({ companyId, onFocus, initialMode = 'capital' }: { companyId: number; onFocus: (id: number | null, mode?: CorrelationMode) => void; initialMode?: CorrelationMode }) {
  // A new origin gets independent selection, filters and expansion state.
  return <FocusedExplorer key={`${companyId}-${initialMode}`} companyId={companyId} onFocus={onFocus} initialMode={initialMode} />
}

function FocusedExplorer({ companyId, onFocus, initialMode }: { companyId: number; onFocus: (id: number | null, mode?: CorrelationMode) => void; initialMode: CorrelationMode }) {
  const [map, setMap] = useState<CorrelationMap | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [retry, setRetry] = useState(0)
  const [mode, setMode] = useState<CorrelationMode>(initialMode)
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [tag, setTag] = useState<string | null>(null)
  const [all, setAll] = useState(false)
  const [expandedView, setExpandedView] = useState(false)
  const [expandedIds, setExpandedIds] = useState<Set<number>>(new Set())
  const theme = useTheme()
  const desktopDetails = useMediaQuery(theme.breakpoints.up('lg'))
  const mobile = useMediaQuery(theme.breakpoints.down('sm'))
  const reducedMotion = useMediaQuery('(prefers-reduced-motion: reduce)')
  const graphRef = useRef<HTMLDivElement>(null)
  const workspaceRef = useRef<HTMLDivElement>(null)
  const returnFocus = useRef<HTMLElement | null>(null)
  const detailRef = useRef<HTMLDivElement>(null)
  const normalScroll = useRef({ left: 0, top: 0 })
  const normalPageScroll = useRef(0)
  const expandButton = useRef<HTMLButtonElement>(null)
  const wasExpanded = useRef(false)

  useEffect(() => {
    const controller = new AbortController()
    setError(null)
    fetchCorrelationMap(companyId, controller.signal).then((data) => {
      if (!controller.signal.aborted) setMap(data)
    }).catch((failure: unknown) => {
      if (!controller.signal.aborted) setError(failure instanceof Error && failure.name !== 'ZodError' ? failure.message : '企業情報を読み込めませんでした。もう一度お試しください。')
    })
    return () => controller.abort()
  }, [companyId, retry])
  useLayoutEffect(() => {
    const workspace = workspaceRef.current
    const main = workspace?.closest('main')
    if (!desktopDetails || !workspace || !main) return
    // 通常表示の図は詳細の下端まで広げ、内容だけを内部スクロールさせる。
    const resize = () => {
      const top = workspace.getBoundingClientRect().top + main.scrollTop
      const minimumHeight = Math.max(280, window.innerHeight - top - 24)
      workspace.style.setProperty('--normal-min-height', `${minimumHeight}px`)
      const height = detailRef.current?.getBoundingClientRect().height ?? minimumHeight
      workspace.style.setProperty('--normal-graph-height', `${height}px`)
    }
    resize()
    const observer = new ResizeObserver(resize)
    observer.observe(main)
    if (detailRef.current) observer.observe(detailRef.current)
    return () => observer.disconnect()
  }, [desktopDetails, map, selectedId, expandedView])
  useLayoutEffect(() => {
    if (expandedView) { wasExpanded.current = true; return }
    const returningFromExpanded = wasExpanded.current
    if (wasExpanded.current) {
      graphRef.current?.scrollTo(normalScroll.current)
      graphRef.current?.closest('main')?.scrollTo({ top: normalPageScroll.current })
      expandButton.current?.focus({ preventScroll: true })
      wasExpanded.current = false
    }
    if (selectedId === null || !desktopDetails || !detailRef.current) return
    revealCompany(graphRef.current, selectedId)
    if (!returningFromExpanded) {
      const main = graphRef.current?.closest('main')
      if (main) {
        const bounds = main.getBoundingClientRect()
        const top = detailRef.current.getBoundingClientRect().top
        if (top < bounds.top || top > bounds.bottom - 100) main.scrollTop += top - bounds.top - 16
      }
    }
    detailRef.current.focus({ preventScroll: true })
  }, [selectedId, desktopDetails, expandedView])

  function select(id: number) {
    returnFocus.current = document.activeElement instanceof HTMLElement ? document.activeElement : null
    setSelectedId(id)
  }
  function close() {
    setSelectedId(null)
    if (returnFocus.current?.isConnected) returnFocus.current.focus()
    graphRef.current?.querySelector<HTMLElement>(`[data-company-id="${selectedId}"]`)?.focus()
  }
  function toggleExpanded(id: number) {
    setExpandedIds((old) => {
      const next = new Set(old)
      if (next.has(id)) next.delete(id); else next.add(id)
      return next
    })
  }
  if (error) return <Alert severity="error" action={<Button onClick={() => setRetry((value) => value + 1)}>再試行</Button>}>{error}<Button onClick={() => onFocus(null)}>企業を選び直す</Button></Alert>
  if (!map) return <PageLoading message="企業のつながりを読み込んでいます…" />
  const focus = map.nodes.find((node) => node.id === companyId)!
  const selected = map.nodes.find((node) => node.id === selectedId)
  // Search all fetched nodes, including folded descendants. Do not infer tags from industry.
  const modeIds = new Set(mode === 'capital' ? map.capital_edges.flatMap((edge) => [edge.parent_id, edge.child_id]) : map.business_edges.flatMap((edge) => [edge.from_id, edge.to_id]))
  const candidates = map.nodes.filter((node) => node.id !== companyId && modeIds.has(node.id))
  const tags = topBusinessTags(candidates)
  const filtered = tag ? sortCompanies(candidates.filter((node) => companyTags(node).includes(tag))) : null
  const focusRelatedCompany = (id: number) => onFocus(id, mode)
  const details = selected && <CompanyDetails key={`${companyId}-${selected.id}-${mode}`} node={selected} map={map} mode={mode} onClose={close} onFocus={focusRelatedCompany} />
  const directCount = (mode === 'capital' ? childrenOf(map, companyId) : partnersOf(map, companyId)).length
  const countControl = !mobile && !tag && directCount > 4 && <div className={styles.countControl}>
    <span role="status">直接つながる{mode === 'capital' ? '子会社' : '取引・協力先'}：{directCount}社中{all ? directCount : 4}社を表示{map.meta.truncated ? '（取得済みの範囲）' : ''}</span>
    <Button aria-expanded={all} onClick={() => setAll((value) => !value)}>{all ? '表示を少なくする' : `残り${directCount - 4}社を見る`}</Button>
  </div>
  const graph = <CompanyGraph map={map} mode={mode} selectedId={selectedId} onSelect={select} onFocus={focusRelatedCompany} all={mobile || all} filtered={filtered} expandedIds={expandedIds} onToggle={toggleExpanded} />

  return <div className={`${styles.explorer} ${!selected && !expandedView ? styles.pageScroll : ''}`}>
    <div className={styles.toolbar}><strong>{focus.name}から探す</strong><Button onClick={() => onFocus(null)}>企業を変更</Button></div>
    <div className={styles.filters}>
      <Tabs value={mode} onChange={(_, value: CorrelationMode) => { setMode(value); setSelectedId(null); setTag(null); setAll(false); setExpandedIds(new Set()); graphRef.current?.scrollTo({ top: 0 }) }} aria-label="会社の関係" variant="fullWidth" className={styles.tabs}>
        <Tab value="capital" label="親会社・子会社" id="capital-tab" aria-controls="correlation-panel" />
        <Tab value="business" label="取引・協力関係" id="business-tab" aria-controls="correlation-panel" />
      </Tabs>
      {tags.length > 0 && <div className={styles.tagFilters} role="group" aria-label="事業分野で絞り込む">
        <Chip label="すべて" onClick={() => setTag(null)} aria-pressed={tag === null} variant={tag === null ? 'filled' : 'outlined'} />
        {tags.map((value) => <Chip key={value} label={value} onClick={() => setTag(tag === value ? null : value)} aria-pressed={tag === value} color={tag === value ? 'primary' : 'default'} variant={tag === value ? 'filled' : 'outlined'} />)}
      </div>}
    </div>
    {map.meta.truncated && <Alert severity="info">関係する企業の一部を表示しています。企業を選び「この企業からつながりを探す」で続きを確認できます。</Alert>}
    <div ref={workspaceRef} className={styles.workspace}>
      <div className={styles.graphColumn}>
        <div className={styles.viewTools}>
          {<Button ref={expandButton} variant="outlined" onClick={() => {
            normalScroll.current = { left: graphRef.current?.scrollLeft ?? 0, top: graphRef.current?.scrollTop ?? 0 }
            normalPageScroll.current = graphRef.current?.closest('main')?.scrollTop ?? 0
            setExpandedView(true)
          }}>拡大表示</Button>}
        </div>
        {countControl}
        <div ref={graphRef} className={styles.graphViewport} role="tabpanel" id="correlation-panel" aria-labelledby={`${mode}-tab`} tabIndex={0}>
          {!expandedView && graph}
        </div>
      </div>
      {selected && desktopDetails && !expandedView && <Box component="aside" ref={detailRef} tabIndex={-1} aria-labelledby="company-details-title" className={styles.detailPanel} onKeyDown={(event) => { if (event.key === 'Escape') close() }}>{details}</Box>}
    </div>
    <ExpandedGraph open={expandedView} onClose={() => setExpandedView(false)} focusId={companyId} countControl={countControl} selectedId={selectedId}
      details={expandedView ? details : null} onCloseDetails={close}
      container={() => graphRef.current?.closest('main')?.parentElement ?? document.body}>
      {expandedView && graph}
    </ExpandedGraph>
    <SwipeableDrawer anchor="bottom" open={!!selected && !desktopDetails && !expandedView} onClose={close} onOpen={() => undefined} disableSwipeToOpen
      transitionDuration={reducedMotion ? 0 : undefined}
      slotProps={{ paper: { className: styles.sheet, role: 'dialog', 'aria-modal': true, 'aria-labelledby': 'company-details-title', sx: { height: '94dvh', maxHeight: 'calc(100dvh - env(safe-area-inset-top) - 8px)', overflow: 'hidden' } } }}>
      <div className={styles.handle} aria-hidden />{!desktopDetails && !expandedView && details}
    </SwipeableDrawer>
  </div>
}
