# STT 評価用の実発話の収集：同意・撤回・削除窓口の設計（SOCAIAGENT-676）

音声認識（STT）を実発話で評価するための、収集ごとの同意・撤回・削除窓口・別バケット・アクセスログの設計。
**設計だけで、実装は後続課題（下の「後続課題」①〜⑤）に分ける。** 声は伏字にできないため、通常の面接データとは別に扱う。

方針の要点は [データプライバシー設計](../wiki/data-privacy.md) の「STT評価用に収集する発話音声」にあり、この文書はそれを実装できる形に落としたもの。
Notion の PRD・DesignDoc（SOCAIAGENT-676）と Figma（同意・撤回／削除依頼／`/privacy` の追記）は下書きで、文面・日付・件数・受付番号・営業日数はすべて仮案。

## 0. 最初に確認した結果（U1）

STT モデルを `gpt-transcribe` に変えた根拠（2026-10-11）に、「実際の面接音声13発話（動画の音声トラック、正解は復元）」がある。

| 確認したところ | 結果 |
|---|---|
| 書かれている場所 | `Backend/internal/openai/transcribe_hints.go` のコメント（88・115行目付近）、`docs/requirements/interview-voice-architecture.md` の 9章の表 |
| 最初に入ったコミット | a924c7e6（PR #1716「未対応issueの一括対応と適性検査第1段」のスカッシュ。185ファイル・16件をまとめた PR） |
| PR #1716 の本文・コメント | 音声の出どころ・同意の根拠の記述は無い |
| リポジトリ（docs・manifest・`real/`） | 13発話の音声・manifest・同意の記録は無い（`real/` は `.gitignore` 済みで、もともとコミットしない） |

**出どころと同意の根拠は確認できなかった。** 「動画の音声トラック」が面接本番の録画（`interview_videos`）なら、
data-privacy.md の「面接本番の録音を後から流用しない」と食い違う。未解決事項の先頭（U1）に置き、収集の実装より先に確かめる。

## 1. 要件

### 1.1 ユーザーストーリー

- **学生として**、収集ごとに同意を自分で決め、いつでも撤回・削除したい。自分の声が知らないうちに使われないように。
- **開発者として**、同意のある実発話だけで誤変換を測りたい。Phase 2 の前に品質ゲート（意味が変わる誤変換が0件）を判定できるように。

### 1.2 受け入れ条件

| Given | When | Then |
|---|---|---|
| 同意していない学生 | 面接や評価を行う | 評価用の音声は1件も保存されない |
| 同意済みで音声が保存されている | 撤回する | 残っている音声がすべて削除され、撤回が履歴に残る |
| 保存から90日経った音声 | 日次ジョブが走る | 削除される |
| 教員・学校管理者・企業 | 評価用の音声を取得しようとする | 403 |
| 同意の文言を変えた（版を上げた） | 旧い版で同意した学生が録音しようとする | 受け付けない（新しい版への同意を求める） |
| 退会した学生 | 退会の処理が走る | 評価用の音声は退会の時点で削除する（12か月の保持の対象にしない） |

### 1.3 非機能要件

- 声は伏字にできない。**別保管・最小件数・最長保存（90日）・アクセスの記録**で守る。
- `user_id` を API のクエリやオブジェクトのキーに付けない（本人は認証から決め、キーはランダム）。
- 評価で外部の AI 事業者（OpenAI の音声認識）へ音声を送ることを、同意の文言に明記する。
- 課金している企業・学校を優遇する要素は無い（収集の対象・件数は条件の表だけで決める）。
- 収集の条件（方言・端末・環境）の偏りは、評価の結論に必ず注記する。

## 2. 全体の流れ

```
[学生] 同意の画面 ──(同意・版つき)──▶ stt_eval_consent_events（追記のみ）
   │
   ├─ 録音（同意済み・今の版のときだけ受け付ける）
   │      └─▶ 別バケット（非公開・暗号化・バージョニングなし・90日で失効）
   │            stt_eval_recordings（メタデータだけ。本文・声は DB に置かない）
   │
   ├─ 撤回 ──▶ 残っている音声をすべて削除（同期で試し、失敗分は日次ジョブが拾う）
   └─ 削除依頼（アプリ内フォーム／メール）──▶ stt_eval_deletion_requests（受付番号）
[音声R&D担当] 一覧・取得（短い期限の署名つき URL）──▶ stt_eval_access_logs
[日次ジョブ] 期限（90日）・撤回・退会・評価完了 の削除 ──▶ 削除の理由を記録
```

## 3. データモデル（案）

スキーマは `Backend/migrations/` に up/down のペアで作る（GORM の AutoMigrate は使わない）。
日時は `training_consent_events` と同じく DB の時計（`NOW(3)`）で入れる。

### 3.1 `stt_eval_consent_events`（同意の履歴。追記のみ）

`training_consent_events`（#2168）に倣う。今の状態は「その学生の id が最大の行」で決める。

| 列 | 型 | 説明 |
|---|---|---|
| `id` | BIGINT UNSIGNED PK | |
| `user_id` | BIGINT UNSIGNED | 学生本人 |
| `granted` | BOOL | 同意（true）／撤回（false） |
| `consent_text_version` | VARCHAR(32) | 操作したときに画面に出していた文言の版 |
| `created_at` | DATETIME(3) | `NOW(3)` |

- 録音を受け付ける条件: 最後の行が `granted = true` で、`consent_text_version` が今の版と一致すること。
- 学校経由の一括同意は作らない（U3）。

### 3.2 `stt_eval_recordings`（音声のメタデータ）

| 列 | 型 | 説明 |
|---|---|---|
| `id` | BIGINT UNSIGNED PK | |
| `user_id` | BIGINT UNSIGNED | 本人（撤回・退会・依頼の削除で引く） |
| `consent_event_id` | BIGINT UNSIGNED | 録音したときの同意の行（どの文言に基づくか） |
| `object_key` | VARCHAR(128) | 別バケットのキー。`stt-eval/<ランダムな UUID>.webm`（`user_id` を入れない） |
| `condition` | VARCHAR(32) | 録音条件（`clean` / `noisy` / `reverb` / `quiet` / `fast` / `hesitation`。`sttbench.Case.Condition` と同じ語） |
| `device` | VARCHAR(32) | 端末の種類（PC内蔵マイク／スマホ など。機種名は持たない） |
| `duration_ms` / `size_bytes` | INT / BIGINT | 件数・容量の上限の判定に使う |
| `expires_at` | DATETIME(3) | 保存から90日 |
| `deleted_at` | DATETIME(3) NULL | 削除した日時 |
| `delete_reason` | VARCHAR(16) NULL | `expired` / `revoked` / `withdrawn` / `request` / `evaluated` |
| `created_at` | DATETIME(3) | |

- 正解の文（読み上げた台本）は台本の ID だけを持ち、本文は持たない。自由発話の書き起こしは PII として**バケットの中にだけ**置き、DB・リポジトリ・ログに出さない。
- 削除しても行は残し（`deleted_at`）、「いつ・なぜ消したか」を説明できるようにする。退会のパージで行ごと消す。

### 3.3 `stt_eval_access_logs`（取得の記録）

| 列 | 型 | 説明 |
|---|---|---|
| `id` | BIGINT UNSIGNED PK | |
| `actor_admin_user_id` | BIGINT UNSIGNED | 操作した音声R&D担当 |
| `action` | VARCHAR(16) | `list` / `download` / `delete` |
| `recording_id` | BIGINT UNSIGNED NULL | 一覧のときは NULL |
| `item_count` | INT | 一覧で返した件数 |
| `created_at` | DATETIME(3) | |

- 記録できなければ取得させない（記録が先・取得が後）。
- 声・書き起こしは持たない。保存期間は1年（案）。

### 3.4 `stt_eval_deletion_requests`（削除依頼の窓口）

| 列 | 型 | 説明 |
|---|---|---|
| `id` | BIGINT UNSIGNED PK | |
| `receipt_no` | VARCHAR(16) | 受付番号（形式は仮案） |
| `user_id` | BIGINT UNSIGNED NULL | アプリ内フォームなら本人。メールで本人を確かめられないうちは NULL |
| `channel` | VARCHAR(8) | `form` / `email` |
| `status` | VARCHAR(16) | `received` / `done` / `no_audio` |
| `deleted_count` | INT | 消した件数 |
| `received_at` / `completed_at` | DATETIME(3) | 完了までの営業日数は仮案 |

## 4. API（案）

`Backend/api/openapi.yaml` に追記する。学生の API は同一オリジンの中継（`frontend/app/api/user/...`）を通す。

| 主体 | メソッド | パス | 内容 |
|---|---|---|---|
| 学生 | GET | `/api/user/stt-eval/consent` | 今の同意の状態・最後に変えた日時・今の文言の版 |
| 学生 | PUT | `/api/user/stt-eval/consent` | `{granted, consent_text_version}`。版が今と違えば 409（古い画面からの同意を記録しない）。撤回なら同じ処理の中で削除を始める |
| 学生 | POST | `/api/user/stt-eval/recordings` | 録音を1件受け付ける。同意が無い・版が古い・上限を超えたら 403 / 409 / 429 |
| 学生 | GET | `/api/user/stt-eval/recordings/summary` | 保存中の件数と、いちばん早い削除予定日（音声そのものは返さない） |
| 学生 | POST | `/api/user/stt-eval/deletion-requests` | 削除依頼。受付番号を返す。音声が無ければ `no_audio` |
| 音声R&D担当 | GET | `/api/admin/stt-eval/recordings?condition=&device=` | 一覧（`user_id` は返さない）。記録してから返す |
| 音声R&D担当 | GET | `/api/admin/stt-eval/recordings/:id/download` | 短い期限（例: 5分）の署名つき URL。記録してから返す |
| 音声R&D担当 | DELETE | `/api/admin/stt-eval/recordings/:id` | 評価が終わった音声を期限より前に消す（`evaluated`） |

- 管理側は**音声R&D担当の権限（新設・案）だけ**が使える。教員・学校管理者・企業ポータルは 403（存在の有無も推測させない）。
- 録音の受付は、学生の行（または最後の同意の行）をロックしてから同意を確かめ、同じトランザクションでメタデータを書く。撤回と重なっても、撤回のあとに録音が残らないようにする（相談機能 #2028 の質問作成と同じ考え方）。

## 5. 削除

| きっかけ | 処理 | 漏れたとき |
|---|---|---|
| 期限（90日） | 日次ジョブが `expires_at` を過ぎた音声を消す | バケットのライフサイクル（90日＋数日で失効）が最後の砦 |
| 撤回 | API の中で消すのを試し、`deleted_at` を入れる | 日次ジョブが「最後の同意が撤回の学生の音声」を消す |
| 退会 | 退会（論理削除）の時点で消す。12か月の保持の対象にしない | 日次ジョブが退会済みの学生の音声を消す。パージで行も消す |
| 削除依頼 | フォームは本人の音声をすべて消して `done`。メールは本人を確かめてから同じ処理 | 担当が受付番号で状況を追う |
| 評価が終わった | 音声R&D担当が消す（`evaluated`） | 期限で消える |

- バケットは**バージョニングを使わない**（消した音声の古い版を残さない）。サーバー側の暗号化・公開アクセスのブロックを必ず付ける。
- 削除の結果は件数と理由だけをログに出す（キー・本文は出さない）。

## 6. 画面（Figma の仮案）

- **収集への同意・撤回**（新規。PC・モバイル）: 未同意／保存中／エラー／同意済み／撤回の確認。面接画面の `ConsentDialog`（面接の録画の同意）とは**別枠**にし、面接の流れの中で同意を求めない。
- **削除依頼**（新規）: 入力／エラー／受付完了（受付番号）／音声なし。
- **`/privacy`**: 4章（保持期間）・5章（ユーザーの権利）・7章（お問い合わせ）に追記し、8章「音声認識の評価のための音声の収集」を新設する。
- 同意の文言に必ず入れること: 開発者が音声を聞くこと／保存期間（最長90日）／**評価で外部の AI 事業者へ音声を送ること**（Figma に未反映）／いつでも撤回・削除を依頼できること／削除の窓口。
- 文言を変えたら版を上げる（`TrainingConsentCard.tsx` の `TRAINING_CONSENT_TEXT_VERSION` と Go 側の定数を揃えるのと同じ運用）。

## 7. 評価（sttbench）への取り込み

- 音声R&D担当が取得した音声と manifest は、リポジトリの外（例: `~/stt-eval-private/`）に置く。manifest は `source: "real"`、`condition` は 3.2 と同じ語にする（`Backend/internal/sttbench/manifest.go`）。
- 結果は件数・誤り率・意味が変わる誤変換の件数だけを残し、書き起こしの本文は結果のファイルにだけ残す（ターミナル・CI のログに出さない）。
- 合成音声の結果と混ぜて比べない。条件の表のうち集まっていない条件は「未評価」と明記する。

## 8. 既存のコードとの対応

| 設計 | 倣う・使うもの |
|---|---|
| 同意の履歴・文言の版 | `models/training_consent_event.go`、`UserPreferenceRepository.SetTrainingUse`、`frontend/components/profile/TrainingConsentCard.tsx` |
| 撤回・退会での削除と日次の削除 | 学習データの書き出しの削除（`Backend/internal/services/training/scorerexport/sweep.go`、[学習データの書き出し](../wiki/scorer-training-data.md)） |
| 退会との連動 | `services/auth/user_deletion_service.go`（パージ）。評価用の音声だけは退会の時点で消す |
| 録音の由来と条件 | `Backend/internal/sttbench/manifest.go` の `Source` / `Condition` |
| 期限での削除の考え方 | 面接動画の保持期間（SOCAIAGENT-671、`20261126214047_school_interview_video_retention`） |
| 画面の告知 | `frontend/app/privacy/page.tsx` |

## 9. 後続課題（実装）

| # | 内容 | 終わりの条件 |
|---|---|---|
| ① | 別バケット・IAM・アクセスの記録（infra） | 書き込みはサーバーだけ、読み取りは音声R&D担当のロールだけ。暗号化・公開ブロック・バージョニングなし・ライフサイクル |
| ② | 同意の記録と API（Backend） | 3.1〜3.4 の migration、4章の API、409 / 403 / 429、テスト |
| ③ | 画面と `/privacy`（Frontend） | 6章の画面、文言の版、`/privacy` の 4・5・7・8 章 |
| ④ | 削除ジョブ・退会との連動 | 5章の表のすべてのきっかけで消え、理由が残る |
| ⑤ | sttbench への取り込み | 7章の手順で、`source: "real"` の結果を合成音声と分けて出す |

コードの変更は `docs/wiki/data-privacy.md` に影響するので、実装の PR で wiki も同じ PR で直す。

## 10. 未解決事項

| # | 内容 | 選択肢と推奨 |
|---|---|---|
| **U1**（最優先） | 13発話の出どころと同意の根拠（0章） | 推奨: 出どころを確かめ、同意が無ければ音声を削除し、モデル変更の根拠から外す（`transcribe_hints.go`・要件定義書の記述を直す）。確認できるまで、この13発話を品質の根拠として使わない |
| U2 | 実発話を集める必要性 | 推奨: 条件ごとに最小限を集める。U1 の確認が先 |
| U3 | 同意の主体 | 推奨: 学生本人。学校経由の一括同意は不可 |
| U4 | 削除窓口 | 推奨: アプリ内フォーム＋メール。メールアドレスは未決 |
| U5 | Phase 2 の前に必須か | 推奨: 必須・最小規模 |
| U6 | 組織をまたぐ収集 | 推奨: 当面不可（自分の組織の学生だけ） |

ほかに決まっていないこと: 収集の件数の上限（条件 × 端末 × 件数）、音声R&D担当の権限の名前と付け方、アクセスの記録の保存期間（案は1年）、受付番号の形式と完了までの営業日数、OpenAI の音声 API でのデータの扱い（学習に使われないこと・保存期間）を同意の文言にどう書くか。

## 参照

- Backlog: SOCAIAGENT-676（依存: SOCAIAGENT-413 / #1603 の⑤、#1247、#1484 / #1485、#2167 / #2168）
- [データプライバシー設計](../wiki/data-privacy.md)「STT評価用に収集する発話音声」
- [AI面接 音声基盤 要件定義書](./interview-voice-architecture.md)
- `docs/research/interview-audio-eval/`（`README.md`・`RESULTS_fallback.md` の 7章）
