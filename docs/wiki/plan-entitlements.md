# 料金プラン × 機能マトリクス（#810）

契約・Stripe は [#612](https://github.com/ISCProduct/soc-ai-agent/issues/612)。本ドキュメントは **機能解放／拒否** の正。

## プラン

| plan_id | 意味 |
|---------|------|
| `free` | 未契約相当 |
| `standard` | 有料（標準） |
| `pro` | 有料（上位） |

環境変数 `DEFAULT_PLAN`（`free` / `standard` / `pro`）で現行プランを stub する。**未設定時は `pro`**（CEATEC デモを落とさない。#612 後は未契約を `free` にする）。

## マトリクス

| 機能 | Feature key | Free | Standard | Pro |
|------|-------------|------|----------|-----|
| 基本マッチング / チャット | `matching` | ○ | ○ | ○ |
| AI 面接 | `interview` | ○（回数上限は #612） | ○ | ○ |
| 職務経歴書 / ES | `resume` | ○ | ○ | ○ |
| 企業相関図 | `company_graph` | × | ○ | ○ |
| 管理画面 | `admin` | × | ○ | ○ |
| API / CSV エクスポート | `export` | × | × | ○ |

コードの定義: `Backend/internal/entitlement/plan.go`（この表と一致させる）。

## API

- `GET /api/entitlements` → `{ plan, features }`
- プラン外は **fail-closed**: `403` + `plan_feature_required`（例: 管理ダッシュボード CSV）

## プラン変更

- アップグレード: 即時（`DEFAULT_PLAN` または将来の subscription）
- ダウングレード: #612 で期末反映を決める
- staging 確認: `DEFAULT_PLAN=free` で Backend を再起動し、CSV エクスポートが 403 / ボタン disabled になること
