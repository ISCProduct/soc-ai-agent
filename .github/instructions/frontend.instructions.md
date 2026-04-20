---
applyTo: "frontend/**/*.{ts,tsx}"
---

Frontend の変更では型安全を最優先し、`any` は避けてください。
Next.js App Router と既存の MUI ベースUIのパターンに合わせて実装してください。
副作用は Hooks に閉じ込め、表示ロジックとデータ処理を分離してください。

