'use client'

import { useEffect, useRef, useState } from 'react'
import Link from 'next/link'
import { Button, CircularProgress, Link as MuiLink } from '@mui/material'
import { Alert } from '@/components/common/Alert'
import OpenInNew from '@mui/icons-material/OpenInNew'
import { businessText, companyTags, fetchCorrelationCompare, marketLabel, safeSourceURL, type CorrelationCompare, type CorrelationMap, type CorrelationMode, type CorrelationNode } from '@/lib/company/correlation-map'
import styles from './explorer.module.css'

export function CompanyDetails({ node, map, mode, onClose, onFocus }: {
  node: CorrelationNode; map: CorrelationMap; mode: CorrelationMode; onClose: () => void; onFocus: (id: number) => void
}) {
  const [showAllRelations, setShowAllRelations] = useState(false)
  const [showComparison, setShowComparison] = useState(false)
  const [comparison, setComparison] = useState<CorrelationCompare | null>(null)
  const [error, setError] = useState(false)
  const [retry, setRetry] = useState(0)
  const [aiNoticeOpen, setAiNoticeOpen] = useState(false)
  const aiHeading = useRef<HTMLHeadingElement>(null)
  useEffect(() => {
    if (aiNoticeOpen) aiHeading.current?.focus()
  }, [aiNoticeOpen])
  const focus = map.nodes.find((company) => company.id === map.focus_company_id)!
  useEffect(() => {
    const controller = new AbortController()
    if (node.id === focus.id) return () => controller.abort()
    setComparison(null)
    setError(false)
    fetchCorrelationCompare(focus.id, node.id, controller.signal)
      .then((data) => { if (!controller.signal.aborted) setComparison(data) })
      .catch(() => { if (!controller.signal.aborted) setError(true) })
    return () => controller.abort()
  }, [focus.id, node.id, retry])
  const capital = map.capital_edges.filter((edge) => edge.parent_id === node.id || edge.child_id === node.id)
  const business = map.business_edges.filter((edge) => edge.from_id === node.id || edge.to_id === node.id)
  const source = safeSourceURL(node.source_url)
  if (aiNoticeOpen) return <div className={styles.details}>
    <header className={`${styles.detailHeader} ${styles.aiHeader}`}>
      <h2 id="company-details-title" ref={aiHeading} tabIndex={-1}>企業研究AI</h2>
      <Button variant="outlined" onClick={onClose}>閉じる</Button>
    </header>
    <div className={styles.detailBody}><p className={styles.aiNotice}>AIチャットは準備中です</p></div>
  </div>
  return <div className={styles.details}>
    <header className={styles.detailHeader}>
      <div><span className={styles.context}>選択中</span><h2 id="company-details-title">{node.name}</h2></div>
      <Button variant="outlined" onClick={onClose}>閉じる</Button>
    </header>
    <div className={styles.detailBody} role="region" aria-label="企業情報の本文" tabIndex={0}>
      <p className={styles.badges}>{companyTags(node).map((tag) => <span className={styles.tag} key={tag}>{tag}</span>)}<span className={styles.market}>{marketLabel(node)}</span></p>
      <section><h3>何をしている会社？</h3><p>{businessText(node)}</p>{node.industry && <p className={styles.context}>業種：{node.industry}</p>}</section>
      <section><h3>会社同士のつながり</h3>
        {(mode === 'capital' ? capital.length : business.length) === 0 && <p>この企業との詳しいつながりはまだ登録されていません。</p>}
        {mode === 'capital' ? (!showAllRelations ? capital.slice(0, 2) : capital).map((edge) => {
          const parent = map.nodes.find((company) => company.id === edge.parent_id)
          const child = map.nodes.find((company) => company.id === edge.child_id)
          if (!parent || !child) return null
          return <div className={styles.relationship} key={edge.id}>
            <p>{edge.relation_type === 'capital_subsidiary' ? `${child.name}は、${parent.name}の子会社として登録されています。` : `${parent.name}と${child.name}には資本関係が登録されています。`}</p>
            <p>{edge.description_known && edge.description.trim() ? edge.description : '事業上の役割や協力内容はまだ登録されていません。'}</p>
          </div>
        }) : (!showAllRelations ? business.slice(0, 2) : business).map((edge) => {
          const other = map.nodes.find((company) => company.id === (edge.from_id === node.id ? edge.to_id : edge.from_id))
          return <div className={styles.relationship} key={edge.id}><p>{other?.name ?? '関連する企業'}との{edge.relation_label || '取引・協力関係'}</p>
            <p>{edge.description_known && edge.description.trim() ? edge.description : '取引・協力内容はまだ登録されていません。'}</p>
          </div>
        })}
        {(mode === 'capital' ? capital.length : business.length) > 2 && <Button fullWidth aria-expanded={showAllRelations} onClick={() => setShowAllRelations((value) => !value)}>{showAllRelations ? 'つながりを折りたたむ' : "残りのつながりを見る（" + ((mode === 'capital' ? capital.length : business.length) - 2) + '件）'}</Button>}
      </section>
      {node.id !== focus.id && <section>{<Button fullWidth aria-expanded={showComparison} onClick={() => setShowComparison((value) => !value)}>起点の企業と比較する</Button>}
        {showComparison && <div>
        {error ? <Alert severity="info" role="alert" action={<Button onClick={() => setRetry((value) => value + 1)}>再試行</Button>}>比較情報を取得できませんでした。</Alert> : comparison ? <>
          {[comparison.focus, comparison.selected].map((company) => <div className={styles.comparison} key={company.id}><h4>{company.name}</h4><p>{businessText(company)}</p></div>)}
        </> : <p role="status"><CircularProgress size={20} aria-hidden /> 比較情報を読み込み中…</p>}
        </div>}
      </section>}
      <details className={styles.hint}><summary>関係を読み解くヒント</summary>
        {mode === 'capital' ? <><p>子会社は、親会社が経営を支配する関係にある会社です。事業内容や仕事、働く条件は会社ごとに異なります。</p>
          <p>出資比率は、その会社への出資の割合を表します。割合だけでは、事業での役割や働き方までは分かりません。</p>
          <p className={styles.context}>親子の向きは登録情報にもとづく表示です。実際の関係は企業の公式情報でも確認してください。</p>
        </> : <p>取引・協力関係は、会社同士が仕事や事業でつながっていることを表します。親会社・子会社とは異なる関係です。</p>}
      </details>
      {source && <MuiLink href={source} target="_blank" rel="noopener noreferrer" className={styles.source}>情報の出典を開く <OpenInNew fontSize="small" /><span className={styles.srOnly}>（新しいタブ）</span></MuiLink>}
      {node.id !== focus.id && <Button variant="outlined" fullWidth onClick={() => onFocus(node.id)}>この企業からつながりを探す</Button>}
    </div>
    <footer className={styles.detailFooter}>
      <Button variant="outlined" fullWidth onClick={() => setAiNoticeOpen(true)} className={styles.aiButton}>
        AIに質問する
      </Button>
      <Button component={Link} href={`/company/${node.id}`} variant="contained" fullWidth>企業詳細を見る</Button>
    </footer>
  </div>
}
