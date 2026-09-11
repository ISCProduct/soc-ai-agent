/** datetime-local 入力と ISO 文字列の往復（ローカル暦日を保持） */

function pad2(n: number): string {
  return String(n).padStart(2, '0')
}

/** Date のローカル暦日を YYYY-MM-DD にする（UTC の toISOString は使わない） */
export function formatLocalDateKey(date: Date): string {
  return `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}`
}

/** ISO UTC 文字列を datetime-local 用 YYYY-MM-DDTHH:mm（ローカル）へ */
export function isoToDatetimeLocal(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return `${formatLocalDateKey(d)}T${pad2(d.getHours())}:${pad2(d.getMinutes())}`
}

/** datetime-local 文字列を ISO UTC へ（ブラウザのローカル解釈に従う） */
export function datetimeLocalToISO(local: string): string {
  if (!local) return ''
  const d = new Date(local)
  if (Number.isNaN(d.getTime())) return ''
  return d.toISOString()
}
