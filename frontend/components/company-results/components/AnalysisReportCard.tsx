import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Award, Code, Target, TrendingUp } from "lucide-react"
import type { UserData } from "../types"

type AnalysisReportCardProps = {
  userData: UserData
}

export function AnalysisReportCard({ userData }: AnalysisReportCardProps) {
  return (
    <Card className="mb-6 border-2 border-primary/20 bg-primary/5">
      <CardHeader>
        <CardTitle className="text-lg flex items-center gap-2">
          <Award className="w-5 h-5" />
          分析レポート - 企業選定条件
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {/* 職種分析 */}
            <div className="space-y-2">
              <div className="flex items-center gap-2 font-semibold text-sm">
                <Code className="w-4 h-4 text-primary" />
                職種分析
              </div>
              <div className="pl-6 space-y-1 text-sm">
                <div>
                  <span className="text-muted-foreground">評価カテゴリ数:</span> {userData.scores?.length || 0}/10
                </div>
              </div>
            </div>

            {/* トップスコア */}
            <div className="space-y-2">
              <div className="flex items-center gap-2 font-semibold text-sm">
                <Target className="w-4 h-4 text-primary" />
                トップ適性
              </div>
              <div className="pl-6 space-y-1 text-sm">
                {userData.scores && userData.scores.length > 0 && (
                  <>
                    {userData.scores
                      .sort((a, b) => b.score - a.score)
                      .slice(0, 3)
                      .map((score, idx) => (
                        <div key={idx}>
                          <span className="text-muted-foreground">{score.category}:</span> {score.score}点
                        </div>
                      ))}
                  </>
                )}
              </div>
            </div>
          </div>

          {/* 診断完了メッセージ */}
          <div className="p-3 bg-green-50 dark:bg-green-900/20 rounded-lg border border-green-200 dark:border-green-800">
            <p className="text-sm text-green-800 dark:text-green-200 text-center">
              ✓ 適性診断が完了しました。あなたに最適な企業をマッチングしています。
            </p>
          </div>

          {/* 診断サマリー */}
          <div className="space-y-2">
            <div className="flex items-center gap-2 font-semibold text-sm">
              <TrendingUp className="w-4 h-4 text-primary" />
              診断サマリー
            </div>
            <div className="pl-6 space-y-1 text-sm">
              <div>
                <span className="text-muted-foreground">総評価カテゴリ:</span> {userData.scores?.length || 0}
              </div>
              <div>
                <span className="text-muted-foreground">最高スコア:</span> {userData.scores && userData.scores.length > 0 ? Math.max(...userData.scores.map(s => s.score)) : 0}点
              </div>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
