import { PageLoading } from '@/components/common/PageLoading'

// 管理画面の遷移先の読み込み中に、すぐ出す（#2084）。管理画面のレイアウト（サイドバー）の中に出るので、
// 学生画面のような全画面の表示・ブランド名にはしない。
export default function Loading() {
  return <PageLoading message="読み込み中..." showBrand={false} fullScreen={false} />
}
