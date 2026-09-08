import PageContent from './page-content'

// 企業情報の投稿フォームは人事ゲスト向けの公開ページ。認証ガードを置かないこと。
// バックエンドも無認証で受け付ける（cmd/server/main.go の /api/company-entry）。
// 14315aa7 の一括 Server Component 化でここに requireSessionUser() が混入し、
// 未ログインの人事が /login へ飛ばされて投稿できなくなっていた（#1074）。
export default function Page() {
  return <PageContent />
}
