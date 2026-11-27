import RouteLoading from '@/components/common/RouteLoading'

// 遷移先の読み込み中に、すぐ画面に出す（#2084）
export default function Loading() {
  return <RouteLoading message="選考スケジュールを読み込み中..." />
}
