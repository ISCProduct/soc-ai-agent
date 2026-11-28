---
name: issue
description: SOC AI Agent の機能や不具合について、Notion の仕様と Backlog 課題の下書き・起票を行う。
---

# 開発課題の起票

`$issue <機能または不具合の概要>` で、課題タイトル、背景、変更範囲、受け入れ条件を整理する。関連する既存課題があれば参照する。

`CLAUDE.md` と `.claude/commands/issue.md` の現行手順に沿って、起票前の確認を行う。仕様の正本には Notion の PRD・DesignDoc・仕様書を使い、文書の形式と保存先は同手順および `docs/notion/prd-designdoc-format.md` を参照する。起票先は Backlog とし、GitHub Issue は既存の連携に任せる。本文作成だけを依頼された場合は下書きを返す。

起票後は課題キーと URL を返す。参照や起票に必要な連携を利用できなければ、その状況と作成済みの下書きを共有する。
