import type { ReactNode } from 'react'

type TabValue = string | number

/**
 * タブ（`<Tab>`）に渡す id と aria-controls。
 *
 * `<TabPanel>` に同じ `idPrefix` と `value` を渡すと、タブと中身が互いを指す
 * （タブの aria-controls → 中身の id、中身の aria-labelledby → タブの id）。
 * `idPrefix` は同じ画面の中で重ならない名前にする。
 */
export function tabA11yProps(idPrefix: string, value: TabValue) {
  return {
    id: `${idPrefix}-tab-${value}`,
    'aria-controls': `${idPrefix}-tabpanel-${value}`,
  }
}

type TabPanelProps = {
  /** `tabA11yProps` に渡したものと同じ接頭辞。 */
  idPrefix: string
  /** この中身に対応するタブの値。 */
  value: TabValue
  /** いま選ばれているタブの値。 */
  selectedValue: TabValue
  children?: ReactNode
}

/**
 * タブの中身（role="tabpanel"）。
 *
 * 選ばれていない中身も要素だけは残し（hidden）、タブの aria-controls が
 * 指す先が常にあるようにする。中身そのものは選ばれているときだけ描画する
 * （選ばれていないタブの図などは読み込まない）。
 * 余白などの見た目は持たない（呼び出し元の見た目を変えないため）。
 */
export function TabPanel({ idPrefix, value, selectedValue, children }: TabPanelProps) {
  const selected = value === selectedValue
  return (
    <div
      role="tabpanel"
      hidden={!selected}
      id={`${idPrefix}-tabpanel-${value}`}
      aria-labelledby={`${idPrefix}-tab-${value}`}
    >
      {selected && children}
    </div>
  )
}
