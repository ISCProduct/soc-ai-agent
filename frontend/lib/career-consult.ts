/**
 * ホームの「キャリア相談」の最初の質問（Figma「改善G／相談中心」）。
 * 押した選択肢の文言は、そのまま回答として送る。バックエンドが相談の意図を扱うようになるまでの暫定の扱い
 * （docs: DesignDoc「4択の回答の扱い」案 A）。
 */
export const CONSULT_LABEL = 'キャリア相談'
export const CONSULT_QUESTION = 'はじめに、いまの気持ちに近いものを選んでください。'
export const CONSULT_OPTIONS = [
  'やりたい仕事を一緒に考えたい',
  '志望する業界について知りたい',
  '就活の進め方がわからない',
  '自分の強みを整理したい',
] as const
export const CONSULT_FREE_INPUT_LABEL = '自分の言葉で入力する'
export const CONSULT_FREE_INPUT_OPTIONAL = '任意'
export const CONSULT_NOTE = '入力した内容にもとづいて、次の質問が変わります。'
