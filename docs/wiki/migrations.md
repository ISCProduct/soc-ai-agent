# DBマイグレーション運用 (#614)

## 概要

DBスキーマは [golang-migrate](https://github.com/golang-migrate/migrate) によるバージョン管理型マイグレーションで管理します。
GORM AutoMigrate は廃止済みで、**スキーマ変更は必ず `Backend/migrations/` のSQLファイルで行います**。

- マイグレーションファイルはバイナリに埋め込まれる（`embed.FS`）ため、デプロイに追加ファイルは不要
- サーバー起動時に未適用のマイグレーションが自動適用される（MySQLの `GET_LOCK` により複数インスタンスの同時起動でも安全）
- 適用状態は `schema_migrations` テーブルで管理される

## Docker Compose での自動適用

ローカル開発（`compose.yml`）では `app` コンテナ起動時に

`Backend/scripts/docker-entrypoint.dev.sh` → `go run ./cmd/migrate up` → `air`

の順でマイグレーションが自動実行されます。

```bash
docker compose up -d --build db app frontend
# または
make core-up
```

dirty 状態で止まっている場合も、当該バージョンのスキーマが揃っていれば自動修復して続行します。
手動確認は次のとおりです。

```bash
docker compose --profile tools run --rm migrate
# またはコンテナ内
docker compose exec app go run ./cmd/migrate version
```

## コマンド（ホストから直接）

```bash
cd Backend
DB_HOST=127.0.0.1 go run ./cmd/migrate            # up: 未適用のマイグレーションをすべて適用（デフォルト）
DB_HOST=127.0.0.1 go run ./cmd/migrate down       # 直近のマイグレーションを1つロールバック
DB_HOST=127.0.0.1 go run ./cmd/migrate version    # 現在のバージョンと dirty フラグを表示
DB_HOST=127.0.0.1 go run ./cmd/migrate force <N>  # バージョンを強制設定（dirty 復旧用・通常は使わない）
```

`SEED_DATA=true go run ./cmd/migrate` で up 後に初期データを投入します。

## スキーマ変更の手順

1. `Backend/migrations/` に連番でファイルを追加する

   ```
   000002_add_xxx_to_users.up.sql    # 変更を適用するSQL
   000002_add_xxx_to_users.down.sql  # 変更を取り消すSQL
   ```

2. **up/down は必ずペアで作成する**（downがないとロールバックできない）
3. GORMモデル（`internal/models/`）も同じPRで変更し、スキーマと一致させる
4. ローカルで `go run ./cmd/migrate` → 動作確認 → `go run ./cmd/migrate down` → 再度 up でロールバック可能性を確認する

### 書き方の注意

- 1ファイル1目的（1テーブルの変更、1インデックスの追加など）に分割する
- データ移行（UPDATE等）とスキーマ変更は別ファイルに分ける
- MySQLのDDLはトランザクションで巻き戻せないため、失敗時は `version`（dirtyフラグ）を確認して手動復旧する（下記）

## ロールバック手順

```bash
# 1. 現在のバージョンを確認
go run ./cmd/migrate version
# => Current version: 2 (dirty: false)

# 2. 直近のマイグレーションを1つ戻す
go run ./cmd/migrate down
# => Current version: 1 (dirty: false)
```

アプリのバージョンも合わせて戻す場合は、旧イメージへの切り戻しとセットで実施します
（旧バイナリには新しいマイグレーションが埋め込まれていないため、DBを先に戻します）。

## dirty 状態からの復旧

マイグレーション実行が途中で失敗すると `dirty: true` になり、以後の適用が止まります。

```bash
go run ./cmd/migrate version
# => Current version: 2 (dirty: true)
```

1. 失敗したマイグレーション（version 2）のSQLがどこまで適用されたかDBを直接確認する
2. 手動でSQLを補修する（残りを適用する or 適用済み分を取り消す）
3. 実際の状態に合わせてバージョンを設定する

```bash
go run ./cmd/migrate force 2   # version 2 が適用済みの状態に補修した場合
go run ./cmd/migrate force 1   # version 2 を取り消した状態に補修した場合
```

## 既存DB（AutoMigrate時代）の移行

`schema_migrations` テーブルが存在せず、かつ `users` テーブルが存在するDBは、
初回の up 実行時に自動的に **version 1（初期スナップショット）適用済み** として記録されます（ベースライン処理）。
既存の本番・開発DBに対して特別な作業は不要です。

## 初期スナップショットについて

`000001_init_schema.up.sql` は、AutoMigrate廃止時点（2026-07-15）のモデル定義を
空のMySQL 8.0に適用し `mysqldump --no-data` から整形して生成したものです（63テーブル）。
以後のスキーマ変更はすべて `000002` 以降の差分ファイルで行います。

## マイグレーション一覧

| Version | 内容 |
|---------|------|
| 1 | 初期スキーマスナップショット |
| 2 | `user_refresh_tokens` |
| 3 | 退会（`withdrawn_at` / `withdrawn_users`） |
| 4 | マルチテナント（`organizations` / memberships / 主要テーブルの `organization_id`）→ [multitenancy.md](./multitenancy.md) |
| 5 | 主要テーブル `organization_id` への FK 制約 |
| 20 | 企業ユーザーの復旧・剥奪（`disabled_at` / トークンのハッシュ化 / タグFKの RESTRICT 化）→ 下の注意を必ず読むこと |
| 22 | マッチング結果の重複行の集約（`user_company_matches`）→ 下の注意を必ず読むこと |
| 23 | `user_company_matches` に一意キー `uniq_user_session_company` |
| 25 | `api_call_logs` に `provider` / `via_fallback`（AIコストの推論先別集計）→ 下の注意を読むこと |
| 26 | `user_company_matches` に `evaluated_categories`（マッチ度の算出軸数）→ 下の注意を読むこと |

### version 26 適用時の注意（#1124）

**適用後、既存のマッチ度は次回の再計算まで古い値のままです。** 列の追加だけで
既存行は書き換えません（`evaluated_categories` は 0 = 不明）。

この変更とセットで**マッチ度の計算式が変わります**。表示される数値が下がります。

| | 変更前 | 変更後 |
| --- | --- | --- |
| カテゴリ別 | シグモイド(k=12)。差20で97.3、差30で91.7 | 線形。差20で80、差30で70 |
| 未計測の軸 | 中立値(50)で埋めて10軸の平均に含める | 平均から除外し、件数を記録 |

本番相当DBの実測（1ユーザー×90社）:

| | 最小 | 中央 | 最大 | 幅 |
| --- | ---: | ---: | ---: | ---: |
| 変更前 | 91.4 | 96.9 | 99.2 | 7.8 |
| 変更後 | 74.7 | 87.2 | 93.7 | 19.0 |

変更前は全90社が91〜99%に固まり「どの企業とも高相性」としか読めなかった。
スコアを1つも持たないユーザーでも全企業と97%前後になる状態だった。

**`evaluated_categories` が小さい行はマッチ度の根拠が薄い**（10軸中2軸で
算出した85%と、10軸すべてで算出した85%は意味が違う）。表示側で示すこと。

### version 25 適用時の注意（#1293）

`api_call_logs` は推論先に関係なく記録されるため、そのままでは「OpenAI への課金額」
として使えませんでした。実測で2つの壊れ方が確認されています。

1. ローカル推論（無料）の行が混ざる。しかも `calculateCost` は未知のモデル名を
   gpt-4o 単価にフォールバックするため、無料の推論が架空コストとして記録される。
2. 企業検索などの通常の OpenAI 利用と同じ財布になる。実データでは 2026-08 の合計が
   $57.99 で、フォールバックの既定月次上限 $20 を恒久的に超過していた
   （= ローカル障害時にフォールバックが一度も発動しない）。

`provider` で課金対象を切り分け、`via_fallback` でフォールバック専用予算を分離します。
`ALGORITHM=INPLACE, LOCK=NONE` なので書き込みは止まりません。

**適用順序に制約があります。アプリより先に version 25 を適用してください。**
アプリを先に出すと `models.APICallLog` に `provider` / `via_fallback` が
含まれるため INSERT が `Unknown column` で全失敗します。`LogUsage` は
エラーをログに出すだけなので気づきにくく、同時に `TotalFallbackCostSince` も
失敗して**フォールバックのUSD上限が fail-open で無効化**されます
（残るブレーキは `OPENAI_FALLBACK_MAX_REQUESTS_PER_MINUTE` だけ）。
コンテナ起動時の自動マイグレーションは無く、`go run ./cmd/migrate up` が必要です。

**既存行は `provider=''` / `via_fallback=0` になります。** `provider` が空の行は
OpenAI 扱い（従来の集計と同じ）です。フォールバック予算の集計は `via_fallback=1` の
行だけなので、適用直後は 0 から始まります。

適用前の確認:

```sql
-- ローカル推論の行が混ざっているか（混ざっていれば架空コストが計上されている）
SELECT model, COUNT(*) c, SUM(cost_usd) usd FROM api_call_logs
 WHERE called_at >= DATE_FORMAT(UTC_TIMESTAMP(), '%Y-%m-01')
 GROUP BY model ORDER BY usd DESC;
```

**down の注意。** 戻すと `via_fallback` が消えるため、フォールバックの USD 上限が
「全コール合計」で判定されます。通常の OpenAI 利用だけで上限に達し、ローカル障害時に
フォールバックが発動しなくなります。戻す場合は
`OPENAI_DAILY_HARD_LIMIT_USD` / `OPENAI_MONTHLY_HARD_LIMIT_USD` を実績に合わせて
引き上げるか、`OPENAI_FALLBACK_ENABLED=false` で明示的に無効化してください。

### version 22 / 23 適用時の注意（#1166）

**重複していたマッチング結果の行が削除されます。** version 23 で
`(user_id, session_id, company_id)` に一意キーを張るため、その前に version 22 で
重複行を1行（`MIN(id)`）へ集約します。

適用前に件数を確認してください。0件なら version 22 はどのUPDATE/DELETEも0行に作用します。

```sql
SELECT COUNT(*) FROM (
  SELECT 1 FROM user_company_matches
   GROUP BY user_id, session_id, company_id HAVING COUNT(*) > 1) x;
```

集約時に失われないよう、以下は残す行へ寄せています。

- `is_viewed` / `is_favorited` / `is_applied`（ユーザー操作の結果）は重複行の `MAX` を採用
- `user_application_statuses.match_id`（FKが無く、参照側は INNER JOIN）は残す行へ付け替え

`match_score` / `match_reason` は残した行の値がそのまま残りますが、次回のマッチング再計算で
upsert により上書きされます。`session_id IS NULL` の行は空文字へ寄せます（Go 側は
`SessionID string` のため常に空文字を書く。NULL のままだと MySQL の UNIQUE が複数 NULL を
許すため一意キーが穴になる）。

version 23 の `ADD UNIQUE KEY` は `LOCK=NONE` のため、ローリングデプロイ中に旧コード
（一意制約を前提としない read-then-write）が重複行を作ると `Error 1062` で失敗し、
`dirty=1` が残ります。**version 22 の4文はいずれも冪等**なので、その場合は
`go run ./cmd/migrate force 22` → `go run ./cmd/migrate up` で再実行すれば収束します。

`session_id` 列は nullable のままです（NOT NULL 化はテーブル再構築を伴うため分離）。
将来モデルを `*string` に変えたりデータインポートを追加すると一意キーが再び穴になるため、
別マイグレーションで `MODIFY session_id varchar(255) NOT NULL DEFAULT ''` を行うのが望ましい。

**ロールバック順序に制約があります。** 一意キー（version 23）が無い状態で新しいアプリを
動かすと、`ON DUPLICATE KEY UPDATE` が衝突を検出できず再計算ごとに重複行が増え続けます。
アプリを戻す場合は `go run ./cmd/migrate down` を先に、ではなく**アプリを先に**旧リビジョンへ
戻してから down を実行してください。集約した行と付け替えた `match_id` は down では復元されません。

### version 20 適用時の注意（#1196）

**未受諾の招待リンクが全て失効します。** 平文の `invite_token` 列を削除するため、
適用時点で受諾されていない招待メールのリンクは動かなくなります。

適用前に件数を確認してください。

```sql
SELECT COUNT(*) FROM company_users
 WHERE invite_token IS NOT NULL AND invite_expires_at > NOW();
```

招待の TTL は24時間（`config.PendingRegistrationTokenTTL`）なので、
デプロイの24時間前から新規招待を止めれば実質ゼロにできます。該当者がいる場合は、
適用後に管理画面から再招待してください（本バージョンから受諾前アカウントの再招待が可能です）。

**ロールバック順序に制約があります。** version 20 適用後にアプリだけ前のリビジョンへ戻すと
500 になります。旧コードの `CompanyUser` は `invite_token` 列を参照するためです。
アプリを戻す場合は `go run ./cmd/migrate down` を必ず同時に実行してください。
なお `down` は列を復元しますが**中身は NULL** なので、いずれにせよ未受諾招待は復活しません。

**企業ユーザーの行削除ができなくなります。** `company_student_tags.created_by` の FK を
`ON DELETE CASCADE` から `RESTRICT` に変更しました。担当者を削除するとその人が付けた
自社タグまで消えるデータ損失を防ぐためです。アクセス剥奪は削除ではなく、
管理画面の無効化（`disabled_at`）で行ってください。
