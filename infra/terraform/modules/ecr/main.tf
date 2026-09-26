resource "aws_ecr_repository" "this" {
  for_each = toset(var.repository_names)

  name                 = each.value
  image_tag_mutability = "MUTABLE"
  force_delete         = var.force_delete

  image_scanning_configuration {
    scan_on_push = true
  }

  tags = merge(var.tags, {
    Name = each.value
  })
}

resource "aws_ecr_lifecycle_policy" "this" {
  for_each = aws_ecr_repository.this

  repository = each.value.name

  # タグ付き（リリース = SHAタグ / staging / buildcache-*）とタグ無しを別ルールにする。
  #
  # 以前は tagStatus=any の1ルールだけで「直近20件」だった。1回のビルドで
  # ECRへ積まれるマニフェストは4件（image index・その中のプラットフォーム manifest・
  # attestation・buildcache）で、うちタグが付くのは2件。残り2件と「タグが移動して
  # 外れた古い index / 古い buildcache」がタグ無しとして溜まり、**タグ無しの山が
  # タグ付きのリリースイメージを枠から押し出していた**。
  # その結果 9/24 の本番イメージが 9/25 には失効し、ECSのサーキットブレーカーが
  # そこへロールバックして CannotPullContainerError で起動不能になった（#1518）。
  #
  # このリポジトリは staging と本番で共用（本番stackはECRを作らない）なので、
  # staging のデプロイ回数もそのまま本番イメージの寿命を削る。
  # タグ付きの枠をタグ無しの増減から切り離すのが要点。
  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "タグ無し（タグが外れた古いindex/buildcache等）は直近${var.lifecycle_untagged_keep_count}件のみ保持"
        selection = {
          tagStatus   = "untagged"
          countType   = "imageCountMoreThan"
          countNumber = var.lifecycle_untagged_keep_count
        }
        action = {
          type = "expire"
        }
      },
      {
        # tagStatus=tagged は tagPrefixList / tagPatternList のどちらかが必須。
        # 本番タグはコミットSHAで共通の接頭辞が無いため tagPatternList=["*"] を使う。
        rulePriority = 2
        description  = "タグ付き（リリース/staging/buildcache）は直近${var.lifecycle_keep_count}件のみ保持"
        selection = {
          tagStatus      = "tagged"
          tagPatternList = ["*"]
          countType      = "imageCountMoreThan"
          countNumber    = var.lifecycle_keep_count
        }
        action = {
          type = "expire"
        }
      }
    ]
  })
}
