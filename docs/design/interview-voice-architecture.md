# AI面接 音声基盤の移行設計

Issue: #1193
PRD: `docs/requirements/interview-voice-architecture.md`

## 概要

AI面接の音声経路には Turn（STT→LLM→TTS）と Realtime の2実装があるが、動いているのは Turn のみで、Realtime はモデル・エンドポイントとも廃止済みで機能しない。本書は現状の構造を明らかにし、案 A（Turn 一本化）／案 B（Realtime 移行）それぞれの実装差分と、両案共通で必要なコスト計測を設計する。

## 現状のアーキテクチャ

```
[Frontend useInterviewSession.ts]
   |
   +-- POST /api/interviews/:id/start-turn  ──┐
   +-- POST /api/interviews/:id/turn        ──┤  実際に使われている
   |                                          │
   |   interview_turn.go                      │
   |     TranscribeWithHints                  │
   |                gpt-4o-mini-transcribe    │
   |       └ 問題発話のみ gpt-4o-transcribe   │
   |                     へ再送(#795 Task 5)  │
   |     ChatInterview    gpt-4o-mini         │
   |     TTS              tts-1               │
   |                                        ──┘
   |
   +-- (未使用) createRealtimeToken  ─────────┐
       POST /api/realtime/token               │  呼び出し元なし
       interview_realtime.go:169              │  かつ動作しない
         model  gpt-4o-realtime-preview  404  │
       openai/realtime.go:41                  │
         POST /v1/realtime/sessions      404  │
                                            ──┘
```

`realtime_usage_logs` / `/admin/costs` は右側の未使用経路のみを対象としている。

## 問題点

| # | 問題 | 根拠 |
| --- | --- | --- |
| 1 | Realtime のモデルが存在しない | `GET /v1/models/gpt-4o-realtime-preview` -> 404 |
| 2 | Realtime のエンドポイントが廃止 | `POST /v1/realtime/sessions` -> `Invalid URL` |
| 3 | フロントに接続実装がない | `createRealtimeToken` の呼び出し元ゼロ |
| 4 | 実使用経路が未計測 | Turn 経路にコスト記録の実装なし |
| 5 | 単価既定値が旧価格 | `realtime_usage_service.go:41-49` |

問題 1〜3 はいずれも「Realtime が実質未完成のまま放置された」ことの現れであり、個別の不具合ではない。

## 案 B: Realtime 移行の設計

### API スキーマ差分

旧 `POST /v1/realtime/sessions` は廃止され、現行は `POST /v1/realtime/client_secrets`。設定項目はすべて `session` 配下へ移り、音声関連は `audio.input` / `audio.output` に整理された。

| 旧（`RealtimeSessionRequest`） | 新（`session` 配下） |
| --- | --- |
| `model` | `session.model` |
| `instructions` | `session.instructions` |
| `modalities: ["audio"]` | `session.output_modalities: ["audio"]` |
| `voice` | `session.audio.output.voice` |
| `input_audio_transcription` | `session.audio.input.transcription` |
| `turn_detection` | `session.audio.input.turn_detection` |
| `max_response_output_tokens` | `session.max_output_tokens` |

現行の設定値をそのまま移した以下のペイロードが HTTP 200 で通ることを実APIで確認済み。

```json
{"session":{"type":"realtime","model":"gpt-realtime-mini",
 "instructions":"...","output_modalities":["audio"],"max_output_tokens":120,
 "audio":{
   "input":{"transcription":{"model":"gpt-4o-mini-transcribe","language":"ja"},
            "turn_detection":{"type":"server_vad","threshold":0.35,
                              "silence_duration_ms":500,"prefix_padding_ms":150,
                              "create_response":true}},
   "output":{"voice":"shimmer"}}}}
```

応答は `{"value":"ek_...","expires_at":...,"session":{...}}`。既存の `RealtimeSessionResponse.ClientSecret.Value` は `value` 直下へ移るため、レスポンス構造体も変更が要る。

**設定の欠落は起きない。** 現行の全項目に対応先がある。

### Backend の変更

- `openai/realtime.go`: エンドポイントを `client_secrets` へ変更。`RealtimeSessionRequest` を `session` ラッパ付きの新構造へ。レスポンス構造体を `value` 直下読みに変更
- `interview_realtime.go:169`: 既定モデルを `gpt-realtime-mini` へ。`OPENAI_REALTIME_MODEL` での上書きは維持
- `interview_realtime.go:171`: 文字起こしモデルは **`gpt-4o-transcribe` のまま据え置く**。
  当初 `gpt-4o-mini-transcribe` へ下げる案だったが、この文字起こしは
  レポート採点の入力になる（要求 F-2）。精度検証（末尾「未検証」）が済むまで下げない。
  費用差は1面接あたり約 $0.03 で、判断を左右する額ではない

同時接続制御 (`CanOpenNewConnection`)、月次アラート、トークン単位のコスト計算は変更不要。

### Frontend の変更

`createRealtimeToken` で取得した ephemeral key を使い WebRTC で接続する処理が新規に必要。

1. `POST /api/realtime/token` で `ek_...` を取得
2. `RTCPeerConnection` を生成し、マイク音声を `addTrack`
3. `ontrack` で受信音声を `<audio>` に接続
4. SDP offer を `POST https://api.openai.com/v1/realtime/calls` へ送信
   - `Content-Type: application/sdp`、ボディは**生の SDP 文字列**
   - `Authorization: Bearer ek_...`（ephemeral key）
   - **モデルはクエリパラメータでもボディでも指定しない。** ephemeral key を発行した
     時点の `session.model` が使われる。クライアントからモデルを差し替えられないため、
     コストの上振れを防ぐ意味でもこの形が正しい
   - 補足: `multipart/form-data` で `sdp` と `session` を送る形式も存在するが、
     それは**バックエンドから通常の API キーで発呼する**経路のもの。本設計は
     ブラウザ + ephemeral key なので該当しない
5. 返却された SDP answer を `setRemoteDescription`
6. データチャネルで発話イベントを受け取り、Backend へ保存（次節）

面接画面は Turn 経路と排他にする。PRD 6.2 のとおり両立はさせない。

### 発話の保存契約（Realtime 採用時）

レポート採点は `InterviewUtterance` を `created_at ASC` で並べた `BuildTranscript` を
入力にする（要求 F-2）。Realtime はイベント駆動で、学生側と面接官側が別イベント・
別タイミングで届くため、保存契約を決めておかないと採点入力が壊れる。

| 保存対象 | 受け取るイベント |
| --- | --- |
| 学生の発話 | `conversation.item.input_audio_transcription.completed` |
| 面接官の発話 | `response.output_audio_transcript.done` |

- **順序**: イベント到着順ではなく、`item_id` / `response_id` で対応付けたターン番号で並べる。
  入力イベントは応答イベントの前後どちらにも到着しうるため、到着順に依存しない
- **冪等性**: `item_id` を一意キーにして保存する。データチャネルの再送や再接続で
  同じイベントが二度届いても行が重複しない
- **中断時の扱い**: `response.done` の `status` で分岐する。
  `completed` は保存、`cancelled`（学生が割り込んだ）は**それまでに確定した
  transcript を保存**、`incomplete`（トークン上限）も同様に保存する。
  いずれも「面接官が実際に発話した内容」なので採点対象に含める

### ロールバック

`OPENAI_REALTIME_MODEL` と経路切替は環境変数で制御し、問題があれば Turn 経路へ戻せる状態で段階展開する。Frontend の経路選択も同じフラグを参照する。

## 案 A: Turn 一本化の設計

削除対象。DB マイグレーションを伴うため expand-contract で進める。

| 対象 | 内容 |
| --- | --- |
| Backend | `interview_realtime.go` の `CreateRealtimeToken`、`openai/realtime.go`、`RealtimeController`、`/api/realtime/*` ルート |
| Frontend | `lib/interview.ts:161` `createRealtimeToken`、`/admin/costs` の Realtime 表示 |
| 計測 | `RealtimeUsageService`、`realtime_usage_repository.go`、`models/realtime_usage_log.go` |
| DB | `realtime_usage_logs` テーブル |

`realtime_usage_logs` の削除は不可逆なので、**先にコード側の参照を落とし、1リリース以上空けてから down SQL 付きでテーブルを落とす**。

**「完了レコードが0件だから空」と判断してはいけない。** `EnsureSessionStarted` は
`status='active'` の行を先に作り、`CloseSession` の更新に失敗しても `FinishSession` は
代替コストを設定して処理を続けるため、`active` のまま残る行が存在しうる。
削除前に全ステータスの件数を確認する。

```sql
SELECT status, COUNT(*) FROM realtime_usage_logs GROUP BY status;
```

0件でなければ、CSV等へエクスポートしてから削除する。down SQL はテーブルを作り直すだけで
**中身は戻らない**。

同時接続制御と月次アラートは Realtime 専用の実装なので、Turn 経路に同等機能が要るかは別途判断する。**この判断を保留したまま削除すると、コスト暴走への防御が無くなる点に注意。**

## 共通: Turn 経路のコスト計測

採用案に関わらず必要。既存の `realtime_usage_logs` を経路非依存の形へ一般化する。

```
interview_usage_logs
  id, user_id, interview_session_id
  route            'turn' | 'realtime'
  model            実際に使ったモデル名。単価改定・モデル変更後に過去分を再計算できるようにする
  duration_seconds

  -- Turn 経路
  stt_seconds, stt_cost_usd
  llm_input_tokens, llm_output_tokens, llm_cost_usd
  tts_characters, tts_cost_usd

  -- Realtime 経路（音声トークンは3種の単価が別なので個別に持つ）
  input_audio_tokens, input_cached_audio_tokens, output_audio_tokens
  input_text_tokens, output_text_tokens

  cost_usd         合計
  status, started_at, ended_at
```

`llm_input_tokens` / `llm_output_tokens` の2列に丸めると、Realtime の
音声入力・キャッシュ済み音声入力・音声出力を区別できず、`RealtimeUsageService.calcCost`
と同じ計算ができない。`cost_usd` がずれると月次アラートの閾値判定もずれるため、
3種を個別の列で持つ。

### 単価

単価はすべて環境変数で上書き可能とし、既定値を現行価格に合わせる。

| 環境変数 | 現行の既定 | 修正後 | 備考 |
| --- | ---: | ---: | --- |
| `REALTIME_AUDIO_INPUT_COST_PER_1M_USD` | 100.0 | 32.0 | `gpt-realtime` |
| `REALTIME_AUDIO_OUTPUT_COST_PER_1M_USD` | 200.0 | 64.0 | `gpt-realtime` |
| `REALTIME_CACHED_AUDIO_INPUT_COST_PER_1M_USD` | 20.0 | 0.40 | `gpt-realtime` |
| `LLM_INPUT_COST_PER_1M_USD` | （新規） | 0.15 | `gpt-4o-mini` |
| `LLM_OUTPUT_COST_PER_1M_USD` | （新規） | 0.60 | `gpt-4o-mini` |
| `STT_COST_PER_MIN_USD` | （新規） | 0.003 | **推定値**。Turnの既定モデル `gpt-4o-mini-transcribe` 用 |
| `TTS_COST_PER_1M_CHARS_USD` | （新規） | 15.0 | `tts-1` |

**モデルと単価の対応を必ず検証する。** 単価は `gpt-realtime` 基準だが、
`OPENAI_REALTIME_MODEL` を `gpt-realtime-mini` に変えると音声入力 $10 /
キャッシュ済み $0.30 / 音声出力 $20 になる。環境変数だけ据え置くと
コスト記録と月次アラートが3倍ずれる。LLM 側も `OPENAI_INTERVIEW_MODEL` で
変わるため同様。**起動時にモデル名と単価の組を突き合わせ、
未知の組み合わせなら警告を出す**（記録した `model` 列で後から再計算もできる）。

**`STT_COST_PER_MIN_USD` は実費ではなく推定値である。** `gpt-4o-mini-transcribe` の
課金は入力音声トークンと出力トークンに基づき、$0.003/分は目安にすぎない
（`gpt-4o-transcribe` へ戻した場合は $0.006/分）。
現在の `Client.Transcribe` はレスポンスから `text` しか返さず usage を捨てているため、
実額は取れない。`/admin/costs` では**推定として明示し、実費と混ぜない**。
実額が必要になったら `Transcribe` の戻り値に usage を足してトークン単位で集計する。

計装点は `interview_turn.go` の3箇所（`:48` Transcribe / `:102` ChatInterview / `:111` TTS）。

## 進め方

不可逆な変更を後ろに置き、案の決定を待たずに着手できるものから進める。

| 段階 | 内容 | 案の決定 |
| --- | --- | --- |
| 1 | 単価既定値を現行価格へ修正 | 不要 |
| 2 | Turn 経路のコスト計測を追加（テーブル一般化・計装） | 不要 |
| 3 | 実測データを1週間収集し、PRD 6.1 のコスト欄を実測値へ更新 | 不要 |
| 4 | 案 A / B の決定 | **必要** |
| 5 | 決定に沿って移行または削除 | — |

段階 1〜3 はどちらの案でも捨てにならない。**段階 4 の判断材料が実測で揃ってから決める。**

## テスト

| 対象 | 方法 |
| --- | --- |
| 単価計算 | `CalcTokenCost` のテーブル駆動テスト。既存 `realtime_usage_service_test.go` に追随 |
| コスト計装 | `httptest.NewServer` で OpenAI をモックし、1面接分の記録が期待値になることを検証 |
| 新API リクエスト生成 | `RealtimeSessionRequest` の JSON が上記スキーマに一致することをゴールデンテストで固定 |
| WebRTC の発呼形式 | `Content-Type: application/sdp` とボディが生の SDP であること、モデルをクエリ・ボディに含めないことを契約テストで固定 |
| 発話の保存 | 学生・面接官の発話が両方保存され、`item_id` での冪等化で再送しても重複しないこと。`cancelled` / `incomplete` でも保存されること |
| Frontend（案 B） | Playwright。面接開始で ephemeral key 取得 → WebRTC 接続確立 → 音声トラック受信までを検証。OpenAI へは接続せずモックする |
| 回帰 | Turn 経路の既存テストが通ること |

案 B の応答遅延（PRD N-1）は自動テストで測れないため、**staging での実測を受け入れ条件とする**。

## 前提として直しておく必要があるもの

- **`POST /api/realtime/token` の IDOR**（#1198 で修正済み）。
  ボディの `user_id` をそのまま `CreateRealtimeToken` へ渡しており、
  `isAllowed` が `actorID == ownerID` で通るため、被害者のIDとセッションIDを
  指定するだけで他人の面接の ephemeral key を発行できた。
  Realtime 経路が現在死んでいるため実害は出ていなかったが、
  **案 B で復旧させる前に塞いでおく必要があった**

## 未検証

- `gpt-realtime-mini` の日本語発話品質。実際に聞いて判断する必要がある
- WebRTC 接続の確立時間。N-1 の 1.0 秒に含めるかは実装後に判断する
- `gpt-4o-mini-transcribe` の日本語文字起こし精度。**合成音声での検証は実施済み**
  （`docs/research/interview-audio-eval/RESULTS_stt_hints.md`）。
  一般指標（CER・固有名詞正解率）では高精度モデルと差が出なかったが、
  **mini は「御社」を8回中8回「本社」と誤認する**。面接では意味が変わるため、
  この語を検知して高精度モデルへ再送するフォールバックを入れた
  （`stt_fallback.go`）。
- **実発話での精度と、フォールバックの再送率が未測定。** 再送率50%が損益分岐点で、
  超えると mini + 再送は高精度モデル単独より高くつく
  （`RESULTS_fallback.md`）。ステージングで `fell_back` を集計して判断する。
  問題があれば `OPENAI_WHISPER_MODEL=gpt-4o-transcribe` へ戻す。
- **戻す経路が本番・ステージングに存在しない。** `OPENAI_WHISPER_MODEL` は
  Terraform のタスク定義（`infra/terraform/environments/*/main.tf` の
  `environment`）に無く、`OPENAI_WEB_SEARCH_MODEL` 等の他モデル名だけが
  明示されている。現状で戻すにはコード変更とデプロイが要る。
  **環境変数の追加は次回のデプロイに合わせて行う**（タスク定義の変更は
  新リビジョン＝サービス更新を伴い、単独で当てると二重タスク稼働になる）
