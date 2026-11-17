# Atlas 設定（#マイグレーション衝突の根治 / Phase 1）
#
# 目的: 既存の golang-migrate 形式ディレクトリ Backend/migrations（6桁連番 up/down）を
# そのまま扱い、Atlas を「オーサリング補助 + CIゲート」としてだけ被せる。
#   - atlas.sum（整合ハッシュ）… 並行PRの採番衝突を git merge conflict として顕在化
#   - atlas migrate lint … 破壊的変更 / データ依存 / 後方非互換 / 順序逆転(non-linear) を検出
#
# ランタイム適用は従来どおり golang-migrate(embed.FS, cmd/migrate, ECS one-off task)。
# Atlas では apply しない（本番適用経路は一切変更しない）。
#
# dev は「解析専用の使い捨てDB」。本番・ローカルの実DBには接続しない。
# docker が必要なのは lint 実行時のみ。atlas migrate hash は DB 不要。

env "migrations" {
  # 使い捨ての解析用 MySQL（本番は mysql:8.0）。atlas が docker で一時起動する。
  dev = "docker://mysql/8/dev"

  migration {
    dir    = "file://Backend/migrations"
    format = golang-migrate
  }
}
