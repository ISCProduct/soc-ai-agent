# SOC AI Agent — ローカル開発用ショートカット (#589)
.PHONY: help rag-up rag-down rag-smoke rag-rebuild core-up

help:
	@echo "Targets:"
	@echo "  make core-up      # db + app + frontend (docker compose up -d で全サービス起動可)"
	@echo "  make rag-up       # chroma + rag-review (--build)"
	@echo "  make rag-smoke    # /health vector_store + chroma heartbeat"
	@echo "  make rag-rebuild  # force-recreate rag-review image"
	@echo "  make rag-down     # stop chroma + rag-review"

core-up:
	docker compose up -d --build db app frontend
	@echo "Migrations run automatically via app entrypoint (see Backend/scripts/docker-entrypoint.dev.sh)"

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
