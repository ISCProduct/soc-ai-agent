---
applyTo: "rag/**/*.py"
---

RAG 側の変更では Python 3.10+ の型ヒントを付与してください。
入力/出力の検証は Pydantic を優先し、ログは `logging` を利用してください。
依存関係追加時は `rag/constraints.txt` を前提に整合性を保ってください。

