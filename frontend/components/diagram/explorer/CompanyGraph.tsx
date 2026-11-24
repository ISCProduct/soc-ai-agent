'use client'

import { useState } from 'react'
import { Button } from '@mui/material'
import { affiliatesOf, childrenOf, parentsOf, partnersOf, type CorrelationMap, type CorrelationMode, type CorrelationNode } from '@/lib/company/correlation-map'
import { CompanyCard } from './CompanyCard'
import styles from './explorer.module.css'

export function CompanyGraph({ map, mode, selectedId, onSelect, onFocus, all, filtered, expandedIds, onToggle }: {
  map: CorrelationMap; mode: CorrelationMode; selectedId: number | null; onSelect: (id: number) => void
  onFocus: (id: number) => void; all: boolean; filtered: CorrelationNode[] | null
  expandedIds?: ReadonlySet<number>; onToggle?: (id: number) => void
}) {
  const [localExpanded, setExpanded] = useState<Set<number>>(new Set())
  const expanded = expandedIds ?? localExpanded
  const focus = map.nodes.find((node) => node.id === map.focus_company_id)!
  const parents = mode === 'capital' ? parentsOf(map, focus.id) : []
  const direct = mode === 'capital' ? childrenOf(map, focus.id) : partnersOf(map, focus.id)
  const affiliates = mode === 'capital' ? affiliatesOf(map, focus.id) : []
  const toggle = onToggle ?? ((id: number) => setExpanded((old) => {
    const next = new Set(old)
    if (next.has(id)) next.delete(id); else next.add(id)
    return next
  }))
  const card = (node: CorrelationNode, context?: string) => <CompanyCard node={node} context={context} origin={node.id === focus.id} selected={node.id === selectedId} onSelect={onSelect} />

  function branch(node: CorrelationNode, path: number[], depth: number) {
    const descendants = childrenOf(map, node.id).filter((child) => !path.includes(child.id))
    const opened = expanded.has(node.id)
    const groupId = `company-children-${path.join('-')}-${node.id}`
    const parentName = map.nodes.find((company) => company.id === path[path.length - 1])?.name
    return <li key={node.id} className={styles.branch}>
      {card(node, `${parentName}の子会社として登録`)}
      {descendants.length > 0 && (depth >= 1 ?
        <Button onClick={() => onFocus(node.id)} className={styles.expand}>{node.name}のつながりを見る</Button> : <>
          <ul id={groupId} hidden={!opened} className={`${styles.branches} ${styles.nested}`}>
            {opened && descendants.map((child) => branch(child, [...path, node.id], depth + 1))}
          </ul>
          <Button onClick={() => toggle(node.id)} aria-expanded={opened} aria-controls={groupId} className={styles.expand}>
            {opened ? '子会社を閉じる' : `子会社${descendants.length}社を見る`}
          </Button>
        </>)}
    </li>
  }

  const shown = all ? direct : direct.slice(0, 4)
  if (filtered) {
    const edges = mode === 'capital'
      ? map.capital_edges.map((edge) => ({ from: edge.parent_id, to: edge.child_id, directed: edge.relation_type === 'capital_subsidiary', label: edge.relation_label }))
      : map.business_edges.map((edge) => ({ from: edge.from_id, to: edge.to_id, directed: edge.direction_known, label: edge.relation_label }))
    const nodes = new Map(map.nodes.map((node) => [node.id, node]))
    const paths = new Map<number, { previous: number; edge: typeof edges[number] }>()
    const queue = [focus.id]
    const visited = new Set(queue)
    for (let index = 0; index < queue.length; index++) {
      const current = queue[index]
      for (const edge of edges) {
        const next = edge.from === current ? edge.to : edge.to === current ? edge.from : null
        if (next === null || visited.has(next) || !nodes.has(next)) continue
        visited.add(next)
        paths.set(next, { previous: current, edge })
        queue.push(next)
      }
    }
    const matches = new Set(filtered.map((node) => node.id))
    const visible = new Set([focus.id])
    for (const node of filtered) {
      if (!visited.has(node.id)) continue
      let current = node.id
      while (!visible.has(current)) {
        visible.add(current)
        current = paths.get(current)!.previous
      }
    }
    function connections(id: number) {
      const children = queue.filter((child) => visible.has(child) && paths.get(child)?.previous === id)
      if (!children.length) return null
      return <ul className={`${styles.branches} ${styles.nested}`} aria-label={`${nodes.get(id)!.name}のつながり`}>
        {children.map((child) => {
          const { edge } = paths.get(child)!
          const from = nodes.get(edge.from)!
          const to = nodes.get(edge.to)!
          const context = mode === 'capital' && edge.directed ? `${to.name}は${from.name}の子会社` : `${from.name}と${to.name}：${edge.label || '取引・協力関係'}`
          return <li key={child} className={`${styles.branch} ${!edge.directed ? styles.undirected : edge.to === id ? styles.reverseArrow : ''}`} data-edge-from={edge.from} data-edge-to={edge.to}>
            {card(nodes.get(child)!, context)}
            {!matches.has(child) && <span className={styles.context}>つながりを示す企業</span>}
            {connections(child)}
          </li>
        })}
      </ul>
    }
    return <div className={`${styles.graph} ${styles.filteredGraph}`}>
      <p className={styles.resultCount} role="status">条件に合う企業：{filtered.length}社{map.meta.truncated ? '（取得範囲内）' : ''}</p>
      {filtered.length === 0 && <p>条件に合う企業がありません。絞り込みを解除して探してください。</p>}
      {filtered.some((node) => !visited.has(node.id)) && <p>起点企業までのつながりが未登録のため、表示できない企業があります。</p>}
      <div className={styles.focus}>{card(focus)}</div>
      {connections(focus.id)}
    </div>
  }
  return <div className={`${styles.graph} ${mode === 'business' ? styles.businessGraph : ''}`}>
    {parents.length > 0 && <div className={`${styles.parents} ${parents.length === 1 ? styles.singleParent : styles.multipleParents}`}>
      {parents.map((node) => <div key={node.id}>{card(node, '親会社として登録')}</div>)}
    </div>}
    <div className={`${styles.focus} ${parents.length ? styles.hasParent : ''}`}>{card(focus)}</div>
    {direct.length > 0 && <>
      <ul className={`${styles.branches} ${shown.length === 1 ? styles.singleBranch : ''}`} aria-label={mode === 'capital' ? '登録されている子会社' : '取引・協力関係のある企業'}>
        {shown.map((node) => mode === 'capital' ? branch(node, [focus.id], 0) : <li key={node.id} className={styles.branch}>
          {card(node, '取引・協力関係')}
        </li>)}
      </ul>
    </>}
    {affiliates.length > 0 && <section className={styles.related}>
      <h3>その他の資本関係</h3>
      <ul className={styles.results}>{affiliates.map((node) => <li key={node.id}>{card(node, '資本関係のある企業')}</li>)}</ul>
    </section>}
  </div>
}

