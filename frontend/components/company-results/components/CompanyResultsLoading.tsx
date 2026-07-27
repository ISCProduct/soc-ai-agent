export function CompanyResultsLoading() {
  return (
    <div className="flex flex-col items-center justify-center py-20">
      <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary mb-4"></div>
      <p className="text-muted-foreground">AI分析結果を取得中...</p>
    </div>
  )
}
