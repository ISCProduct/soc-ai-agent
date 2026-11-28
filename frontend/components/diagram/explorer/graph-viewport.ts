/** ページ全体を動かさず、図の表示領域内に選択したカードを収める。 */
export function revealCompany(viewport: HTMLElement | null, id: number | null, centerHorizontally = false) {
  if (!viewport || id === null) return
  const card = viewport.querySelector<HTMLElement>(`[data-company-id="${id}"]`)
  if (!card) return
  const bounds = viewport.getBoundingClientRect()
  const rect = card.getBoundingClientRect()
  const availableWidth = viewport.clientWidth - 16
  const availableHeight = viewport.clientHeight - 16
  const left = centerHorizontally ? rect.left - bounds.left - (viewport.clientWidth - rect.width) / 2
    : rect.left < bounds.left + 8 || rect.width > availableWidth ? rect.left - bounds.left - 8
      : Math.max(0, rect.right - bounds.left - viewport.clientWidth + 8)
  const top = rect.top < bounds.top + 8 || rect.height > availableHeight ? rect.top - bounds.top - 8
    : Math.max(0, rect.bottom - bounds.top - viewport.clientHeight + 8)
  viewport.scrollBy({ left, top, behavior: 'instant' })
}
