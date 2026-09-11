# MCP resume review environment

This compose overlay adds MCP servers for:
- pdf-reader-mcp (resume PDF text extraction)
- brave-search (company research via Brave Search API)

## Notion MCP（ドキュメント同期）

Cursor から Notion にページを作成するには、公式 Notion MCP を接続します。

1. プロジェクトに `.cursor/mcp.json` を配置済み（`https://mcp.notion.com/mcp`）
2. **Cursor を再起動**
3. **Settings → MCP** で `notion` が表示されたら **Connect / Needs login** から OAuth 完了
4. 接続後、チャットで「Notion にドキュメントを作成して」と依頼

作成対象（`docs/notion/`）:
- `617-redis-rate-limit-jobs.md`
- `infra-decision-aws-staging-prod.md`

## Prerequisites
- Docker + Docker Compose
- Brave Search API key

## Environment variables

Set these in your shell or a `.env` file before starting:

```bash
BRAVE_API_KEY=your-brave-api-key
```

Optional overrides:

```bash
MCP_PDF_READER_IMAGE=ghcr.io/modelcontextprotocol/servers/pdf-reader:latest
MCP_BRAVE_SEARCH_IMAGE=ghcr.io/modelcontextprotocol/servers/brave-search:latest
MCP_PDF_READER_PORT=7001
MCP_BRAVE_SEARCH_PORT=7002
MCP_PDF_READER_TRANSPORT=sse
MCP_BRAVE_SEARCH_TRANSPORT=sse
```

## Start

```bash
docker compose -f compose.yml -f compose.mcp.yml up -d
```

The MCP servers will be available on:
- `http://localhost:7001` (pdf-reader-mcp)
- `http://localhost:7002` (brave-search)

## Notes
- If your MCP server images expose a different port or transport, override the vars above.
- Keep `BRAVE_API_KEY` out of source control.
