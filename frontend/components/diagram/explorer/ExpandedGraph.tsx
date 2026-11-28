'use client'

import { useLayoutEffect, useRef, useState, type PointerEvent, type ReactNode } from 'react'
import { Box, Button, ButtonBase, Dialog, DialogTitle, IconButton, Tooltip } from '@mui/material'
import Add from '@mui/icons-material/Add'
import Remove from '@mui/icons-material/Remove'
import ArrowBack from '@mui/icons-material/ArrowBack'
import RestartAlt from '@mui/icons-material/RestartAlt'
import CenterFocusStrongOutlined from '@mui/icons-material/CenterFocusStrongOutlined'
import PanToolOutlined from '@mui/icons-material/PanToolOutlined'
import { revealCompany } from './graph-viewport'
import styles from './explorer.module.css'

export function ExpandedGraph({ open, onClose, focusId, children, countControl, selectedId, details, onCloseDetails, container, title = '企業相関図' }: {
  open: boolean; onClose: () => void; focusId: number; children: ReactNode
  countControl: ReactNode; selectedId: number | null; details: ReactNode; onCloseDetails: () => void
  container: () => HTMLElement; title?: string
}) {
  const [zoom, setZoom] = useState(100)
  const viewport = useRef<HTMLDivElement>(null)
  const canvas = useRef<HTMLDivElement>(null)
  const detailPanel = useRef<HTMLElement>(null)
  const drag = useRef<{ id: number; x: number; y: number; left: number; top: number } | null>(null)

  useLayoutEffect(() => {
    if (!open || selectedId === null) return
    revealCompany(viewport.current, selectedId, true)
    detailPanel.current?.scrollTo({ top: 0 })
    detailPanel.current?.focus({ preventScroll: true })
  }, [open, selectedId])

  function enterDiagram() {
    if (!viewport.current || !canvas.current) return
    // 詳細欄の開閉でカードを並べ直さず、同じキャンバス内を移動する。
    canvas.current.style.width = `${Math.max(856, viewport.current.clientWidth)}px`
    if (selectedId !== null) {
      revealCompany(viewport.current, selectedId, true)
      detailPanel.current?.focus({ preventScroll: true })
    } else goToOrigin()
  }

  function goToOrigin() {
    const area = viewport.current
    const origin = canvas.current?.querySelector<HTMLElement>(`[data-company-id="${focusId}"]`)
    if (!area || !origin) return
    const a = area.getBoundingClientRect(), b = origin.getBoundingClientRect()
    area.scrollTo({ left: area.scrollLeft + b.x - a.x - (area.clientWidth - b.width) / 2,
      top: area.scrollTop + b.y - a.y - Math.min(64, area.clientHeight / 4), behavior: 'instant' })
  }
  function changeZoom(next: number) {
    const area = viewport.current, content = canvas.current
    if (!area || !content) return
    const bounded = Math.max(50, Math.min(200, next)), ratio = bounded / zoom
    const left = (area.scrollLeft + area.clientWidth / 2) * ratio - area.clientWidth / 2
    const top = (area.scrollTop + area.clientHeight / 2) * ratio - area.clientHeight / 2
    // レイアウト寸法も倍率に合わせ、スクロールで図の端まで届くようにする。
    content.style.zoom = String(bounded / 100)
    setZoom(bounded)
    area.scrollTo({ left, top, behavior: 'instant' })
  }
  function startDrag(event: PointerEvent<HTMLDivElement>) {
    if (event.button !== 0 || event.pointerType === 'touch' || (event.target as HTMLElement).closest('button,a,summary')) return
    const area = event.currentTarget
    drag.current = { id: event.pointerId, x: event.clientX, y: event.clientY, left: area.scrollLeft, top: area.scrollTop }
    area.setPointerCapture(event.pointerId)
    area.dataset.dragging = 'true'
    event.preventDefault()
  }
  function moveDrag(event: PointerEvent<HTMLDivElement>) {
    const start = drag.current
    if (!start || start.id !== event.pointerId) return
    event.currentTarget.scrollLeft = start.left - event.clientX + start.x
    event.currentTarget.scrollTop = start.top - event.clientY + start.y
  }
  function endDrag(event: PointerEvent<HTMLDivElement>) {
    drag.current = null
    delete event.currentTarget.dataset.dragging
    if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId)
  }
  // 通常画面の詳細シートと拡大画面のフォーカス制御を重ねない。
  if (!open) return null
  return <Dialog open fullScreen disableRestoreFocus transitionDuration={0} onClose={onClose} container={container} aria-labelledby="expanded-graph-title"
    slotProps={{ transition: { onEntered: enterDiagram }, paper: { className: styles.expandedDialog } }}>
    <div className={styles.expandedHeader}>
      <Button startIcon={<ArrowBack />} aria-label="通常の相関図に戻る" onClick={onClose}>戻る</Button>
      <DialogTitle id="expanded-graph-title" sx={{ p: 0, fontSize: 20, fontWeight: 700 }}>{title}</DialogTitle>
    </div>
    <div className={styles.zoomTools} role="group" aria-label="図の表示操作">
      <div className={styles.zoomControls}>
        <div className={styles.zoomGroup} role="group" aria-label="拡大・縮小">
          <IconButton aria-label="図を縮小" disabled={zoom <= 50} onClick={() => changeZoom(zoom - 25)}><Remove fontSize="small" /></IconButton>
          <Tooltip title="100％に戻す" describeChild>
            <ButtonBase className={styles.zoomReset} aria-label="100％に戻す" onClick={() => changeZoom(100)}>
              <output aria-live="polite" aria-label="表示倍率">{zoom}％</output><RestartAlt className={styles.resetIcon} />
            </ButtonBase>
          </Tooltip>
          <IconButton aria-label="図を拡大" disabled={zoom >= 200} onClick={() => changeZoom(zoom + 25)}><Add fontSize="small" /></IconButton>
        </div>
        <span className={styles.zoomDivider} aria-hidden="true" />
        <Button className={styles.originButton} startIcon={<CenterFocusStrongOutlined />} aria-label="起点の会社へ戻る" onClick={goToOrigin}>起点へ戻る</Button>
      </div>
      <p id="graph-navigation-help" className={styles.expandedHelp}><PanToolOutlined aria-hidden="true" />余白をドラッグ・スワイプで移動<span className={styles.srOnly}>。企業を選ぶと詳細を開けます。</span></p>
    </div>
    <div className={styles.expandedCounts}>{countControl}</div>
    <div className={styles.expandedWorkspace}>
      <Box ref={viewport} className={styles.expandedViewport} tabIndex={0} role="region" aria-label="拡大した相関図" aria-describedby="graph-navigation-help"
      onPointerDown={startDrag} onPointerMove={moveDrag} onPointerUp={endDrag} onPointerCancel={endDrag} onLostPointerCapture={(event) => { drag.current = null; delete event.currentTarget.dataset.dragging }}>
      <div ref={canvas} className={styles.expandedCanvas} style={{ zoom: zoom / 100 }}>{children}</div>
      </Box>
      {details && <aside ref={detailPanel} className={`${styles.detailPanel} ${styles.expandedDetails}`} aria-labelledby="company-details-title" tabIndex={-1}
        onKeyDown={(event) => { if (event.key === 'Escape') { event.stopPropagation(); onCloseDetails() } }}>{details}</aside>}
    </div>
  </Dialog>
}
