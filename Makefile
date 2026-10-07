# SOC AI Agent — ローカル開発用ショートカット (#589)
.PHONY: help rag-up rag-down rag-smoke rag-rebuild core-up api-docs api-docs-down api-catalog prod-status prod-logs prod-logs-follow prod-logs-errors

help:
	@echo "Targets:"
	@echo "  make core-up      # db + app + frontend (docker compose up -d で全サービス起動可)"
	@echo "  make rag-up       # chroma + rag-review (--build)"
	@echo "  make rag-smoke    # /health vector_store + chroma heartbeat"
	@echo "  make rag-rebuild  # force-recreate rag-review image"
	@echo "  make rag-down     # stop chroma + rag-review"
	@echo ""
	@echo "API ドキュメント:"
	@echo "  make api-docs      # Swagger UI を起動 (http://localhost:8081)"
	@echo "  make api-docs-down # Swagger UI を停止"
	@echo "  make api-catalog   # routes.txt を実装に合わせて更新し網羅率を出す"
	@echo ""
	@echo "本番(AWS ECS)のログ:"
	@echo "  make prod-status        # サービス稼働状態 + ログ最終書き込み時刻"
	@echo "  make prod-logs          # 直近ログ (SVC=backend|frontend|rag|chroma|all, SINCE=1h)"
	@echo "  make prod-logs-follow   # ライブ追尾"
	@echo "  make prod-logs-errors   # エラー行のみ抽出"

core-up:
	docker compose up -d --build db app frontend
	@echo "Migrations run automatically via app entrypoint (see Backend/scripts/docker-entrypoint.dev.sh)"

api-docs:
	docker compose --profile docs up -d swagger-ui
	@echo "Swagger UI: http://localhost:$${SWAGGER_UI_PORT:-8081}"

api-docs-down:
	docker compose --profile docs down swagger-ui

# routes.txt は実装から生成する。openapi.yaml は手で書くので触らない。
api-catalog:
	cd Backend && go test ./internal/routes/ -run TestRouteCatalog -update
	cd Backend && go test ./internal/routes/ -v -run TestOpenAPIPaths 2>&1 | grep -E '網羅率|FAIL' || true

rag-up:
	./scripts/dev-rag-up.sh

rag-down:
	docker compose stop chroma rag-review

rag-rebuild:
	docker compose up -d --build --force-recreate chroma rag-review
	$(MAKE) rag-smoke

rag-smoke:
	@echo "Chroma:" && curl -sf http://127.0.0.1:8000/api/v2/heartbeat && echo
	@echo "RAG health:" && curl -sf http://127.0.0.1:9000/health && echo
	@echo "RAG vector/status:" && curl -sf http://127.0.0.1:9000/vector/status && echo
	@curl -sf http://127.0.0.1:9000/health | grep -q '"vector_store"' || (echo "旧イメージの可能性: make rag-rebuild" && exit 1)

# ── 本番(AWS ECS on Fargate)のログ ──────────────────────────────
# 実体は aws logs tail のラッパー。詳細は ./scripts/prod-logs.sh --help
SVC   ?= backend
SINCE ?= 1h

prod-status:
	./scripts/prod-logs.sh status

prod-logs:
	./scripts/prod-logs.sh $(SVC) --since $(SINCE)

prod-logs-follow:
	./scripts/prod-logs.sh $(SVC) --follow

prod-logs-errors:
	./scripts/prod-logs.sh $(SVC) --since $(SINCE) --filter-pattern "?ERROR ?error ?Failed ?panic"
