'use client'

import { ButtonBase } from '@mui/material'
import { companyTags, marketLabel, type CorrelationNode } from '@/lib/company/correlation-map'
import styles from './explorer.module.css'

export function CompanyCard({ node, origin = false, selected = false, context, onSelect }: {
  node: CorrelationNode; origin?: boolean; selected?: boolean; context?: string; onSelect: (id: number) => void
}) {
  return (
    <ButtonBase onClick={() => onSelect(node.id)} aria-label={`${node.name}の情報を見る${origin ? '（起点）' : ''}`}
      data-company-id={node.id} aria-pressed={selected} className={`${styles.card} ${origin ? styles.origin : ''} ${selected ? styles.selected : ''}`}>
      {context && <span className={styles.context}>{context}</span>}
      <span className={styles.cardTitle}>{node.name}</span>
      {selected && <span className={styles.srOnly}>選択中</span>}
      <span className={styles.business}>{node.short_business_known && node.short_business.trim() ? node.short_business : '事業内容はまだ登録されていません。'}</span>
      <span className={styles.badges}>
        {companyTags(node).slice(0, 2).map((tag) => <span className={styles.tag} key={tag}>{tag}</span>)}
        <span className={styles.market}>{marketLabel(node)}</span>
      </span>
    </ButtonBase>
  )
}
