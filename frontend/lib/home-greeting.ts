/** ホームの挨拶に使う姓。「山田 太郎」（空白区切り）から先頭の語を返す。空白が無い名前はそのまま返す。 */
export function familyNameOf(name: string | null | undefined): string {
  const trimmed = (name ?? '').trim()
  if (!trimmed) return ''
  return trimmed.split(/[\s　]+/)[0]
}

/** 「{姓}さん、こんにちは」。名前が無いときは「こんにちは」。 */
export function greetingFor(name: string | null | undefined): string {
  const family = familyNameOf(name)
  return family ? `${family}さん、こんにちは` : 'こんにちは'
}
