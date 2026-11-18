/**
 * ホームの「キャリア相談」の最初の質問（Figma「改善G／相談中心」）。
 * 押した選択肢の文言は回答として送り、あわせて意図（consult_intent）を送る。
 * バックエンドは意図を保存し、次の質問で職種を聞く（SOCAIAGENT-668、案 B）。
 */
/** バックエンドの ConsultIntent*（Backend/internal/services/chat/consult_intent.go）と値を合わせる */
export type ConsultIntent = 'find_job' | 'learn_industry' | 'how_to_proceed' | 'organize_strengths' | 'free'
export const CONSULT_LABEL = 'キャリア相談'
export const CONSULT_QUESTION = 'はじめに、いまの気持ちに近いものを選んでください。'
export const CONSULT_OPTIONS = [
  'やりたい仕事を一緒に考えたい',
  '志望する業界について知りたい',
  '就活の進め方がわからない',
  '自分の強みを整理したい',
] as const
/** 選択肢の文言から、送る意図を引く。自由入力は 'free'。 */
export const CONSULT_INTENT_BY_OPTION: Record<(typeof CONSULT_OPTIONS)[number], ConsultIntent> = {
  やりたい仕事を一緒に考えたい: 'find_job',
  志望する業界について知りたい: 'learn_industry',
  就活の進め方がわからない: 'how_to_proceed',
  自分の強みを整理したい: 'organize_strengths',
}
export const CONSULT_FREE_INPUT_LABEL = '自分の言葉で入力する'
export const CONSULT_FREE_INPUT_OPTIONAL = '任意'
export const CONSULT_NOTE = '入力した内容にもとづいて、次の質問が変わります。'
