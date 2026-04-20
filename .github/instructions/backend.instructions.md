---
applyTo: "Backend/**/*.go"
---

Backend の変更では、DDD の層分離（Controller / Service / Repository / Model）を維持してください。
ユースケース実装は Service 層に寄せ、Repository 層にビジネスロジックを混在させないでください。
エラー処理は `errors.Is/As` を意識し、曖昧な握りつぶしを避けてください。

