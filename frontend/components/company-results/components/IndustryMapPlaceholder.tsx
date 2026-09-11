import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

export function IndustryMapPlaceholder() {
  return (
    <Card className="mb-6 border-2 border-dashed">
      <CardHeader>
        <CardTitle className="text-lg">選定企業の業界マップ（開発中）</CardTitle>
        <CardDescription>資本関連図とビジネス関連図を表示予定</CardDescription>
      </CardHeader>
      <CardContent>
        <p className="text-sm text-muted-foreground">
          将来的に、選定企業の資本関係やビジネス関係を可視化した業界マップを表示します。
        </p>
      </CardContent>
    </Card>
  )
}
