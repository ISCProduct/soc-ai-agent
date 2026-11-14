# SOC AI Agent — ローカル開発用ショートカット (#589)
.PHONY: help rag-up rag-down rag-smoke rag-rebuild core-up migrate-new migrate-hash migrate-lint api-docs api-docs-down api-catalog prod-status prod-logs prod-logs-follow prod-logs-errors test-e2e-up test-e2e-seed test-e2e test-e2e-down live2d-up live2d-authoring

# ローカル統合テスト環境（個人の dev スタックと分離・同時起動可。compose.test.yml）
TEST_COMPOSE := docker compose -p soc-ai-agent-test -f compose.yml -f compose.test.yml
TEST_BACKEND_PORT ?= 18080
TEST_FRONTEND_PORT ?= 13000

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
	@echo ""
	@echo "Live2D面接官 (資産は非公開S3から取る。docs/wiki/live2d-assets.md):"
	@echo "  make live2d-up        # 実行時資産を取得し、.env.localのフラグを立てて検査する"
	@echo "  make live2d-authoring # 制作用の元絵とCubismプロジェクトを取得（描き直す人だけ）"
	@echo ""
	@echo "ローカル統合テスト環境 (個人のdevスタックと同時起動可。docs/wiki/getting-started.md):"
	@echo "  make test-e2e-up    # db/redis/app/frontend を分離した専用スタックで起動"
	@echo "  make test-e2e-seed  # 固定の学生ユーザーを冪等にシード"
	@echo "  make test-e2e       # up -> seed -> smoke E2E を一括実行"
	@echo "  make test-e2e-down  # 専用スタックを停止（ボリュームは残す）"

test-e2e-up:
	@test -f Backend/.env.test || (echo "Backend/.env.test が無い。'cp Backend/.env.test.example Backend/.env.test' を実行してください" && exit 1)
	$(TEST_COMPOSE) up -d --build db redis app frontend
	@printf "Backend起動待ち"; \
	for i in $$(seq 1 60); do \
		curl -sf http://127.0.0.1:$(TEST_BACKEND_PORT)/healthz >/dev/null 2>&1 && \
		curl -sf http://127.0.0.1:$(TEST_FRONTEND_PORT)/api/healthz >/dev/null 2>&1 && echo " OK" && exit 0; \
		printf "."; sleep 2; \
	done; \
	echo " タイムアウト。'docker compose -p soc-ai-agent-test logs app frontend' を確認してください" && exit 1

test-e2e-seed:
	cd Backend && go run ./cmd/seed-e2e

# 並列make(-j)でも up -> seed の順を守るため、seed は up の後に再帰makeで呼ぶ。
test-e2e: test-e2e-up
	$(MAKE) test-e2e-seed
	cd frontend && E2E_LOCAL_INTEGRATION=true PLAYWRIGHT_BASE_URL=http://localhost:$(TEST_FRONTEND_PORT) PLAYWRIGHT_API_BASE_URL=http://localhost:$(TEST_BACKEND_PORT) npx playwright test --config=playwright.smoke.config.ts

test-e2e-down:
	$(TEST_COMPOSE) down

core-up:
	docker compose up -d --build db app frontend
	@echo "Migrations run automatically via app entrypoint (see Backend/scripts/docker-entrypoint.dev.sh)"

# 新しいマイグレーションを作成する（日本時間(JST)タイムスタンプ採番）。
# 連番ではなくタイムスタンプを使うことで、並行ブランチが「次の番号」を取り合わない
# （採番衝突の根治）。既存の6桁連番(000001〜)はそのまま。golang-migrate は数値順に
# 適用するため、タイムスタンプ(14桁)は常に6桁の後に適用される。
# TZ を Asia/Tokyo に固定し、各自のマシン設定に依存せず必ず JST で採番する
# （全員が同一TZで一貫採番することが数値順＝作成順を保つ条件。日本は夏時間なし）。
# 使い方: make migrate-new name=add_foo_to_users
migrate-new:
	@test -n "$(name)" || { echo "usage: make migrate-new name=<snake_case_name>"; exit 1; }
	@ts=$$(TZ='Asia/Tokyo' date +%Y%m%d%H%M%S); \
	up="Backend/migrations/$${ts}_$(name).up.sql"; \
	down="Backend/migrations/$${ts}_$(name).down.sql"; \
	if [ -e "$$up" ]; then echo "already exists: $$up (1秒おいて再実行)"; exit 1; fi; \
	touch "$$up" "$$down"; \
	echo "created: $$up"; echo "created: $$down"; \
	atlas migrate hash --env migrations; \
	echo "atlas.sum を更新しました。SQLを書いたら up/down/atlas.sum を同じPRでコミットしてください"

# マイグレーション整合ハッシュ(atlas.sum)を再計算する。
# Backend/migrations/ に up/down を追加・変更したら必ず実行し、atlas.sum を同じPRでコミットする。
# DBは不要（ファイルのチェックサムのみ）。atlas の Community 版が要る（docs/wiki/migrations.md 参照）。
migrate-hash:
	atlas migrate hash --env migrations

# マイグレーションを lint する（破壊的変更/データ依存/後方非互換/順序逆転を検出）。
# 解析用の使い捨てMySQLを docker で起動するため Docker が必要。origin/develop との差分を見る。
migrate-lint:
	git fetch --quiet origin develop
	atlas migrate lint --env migrations --git-base origin/develop

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

# ── Live2D面接官の資産 ──────────────────────────────────────────
# Core とモデルは public リポジトリに置けないため非公開S3から取る。
# 理由と配り方は docs/wiki/live2d-assets.md。
LIVE2D_ASSETS_BUCKET ?= soc-build-assets-508897596159

live2d-up:
	LIVE2D_ASSETS_BUCKET=$(LIVE2D_ASSETS_BUCKET) ./automation/ops/fetch-live2d-assets.sh
	@# 既存行が false でも true に直す。無いときだけ追記すると、一度無効にした .env.local では面接官が Avatar2D のままになる。
	@if [ -f frontend/.env.local ] && grep -qs '^NEXT_PUBLIC_LIVE2D_ENABLED=' frontend/.env.local; then \
		sed -i.bak 's/^NEXT_PUBLIC_LIVE2D_ENABLED=.*/NEXT_PUBLIC_LIVE2D_ENABLED=true/' frontend/.env.local && rm -f frontend/.env.local.bak; \
	else \
		printf '\n# Live2D面接官を有効にする（make live2d-up が追記）\nNEXT_PUBLIC_LIVE2D_ENABLED=true\n' >> frontend/.env.local; \
	fi
	@grep -n '^NEXT_PUBLIC_LIVE2D_ENABLED=' frontend/.env.local
	cd frontend && npm run check:live2d -- public/live2d/interviewers/male
	cd frontend && npm run check:live2d -- public/live2d/interviewers/female
	@echo ""
	@echo "準備できました。npm run dev を動かし直してから /interview を開いてください。"
	@echo "NEXT_PUBLIC_* はビルド時に焼き込まれるので、起動中のdevサーバーには反映されません。"

live2d-authoring:
	LIVE2D_ASSETS_BUCKET=$(LIVE2D_ASSETS_BUCKET) ./automation/ops/fetch-live2d-authoring.sh

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
