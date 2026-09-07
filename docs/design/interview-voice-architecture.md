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
   |     :48  Transcribe   gpt-4o-transcribe  │
   |     :102 ChatInterview gpt-4o-mini       │
   |     :111 TTS          tts-1              │
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
- `interview_realtime.go:171`: 文字起こしモデルの既定を `gpt-4o-mini-transcribe` へ（Realtime 経路では応答生成に使われないため、精度要求が Turn 経路より低い）

同時接続制御 (`CanOpenNewConnection`)、月次アラート、トークン単位のコスト計算は変更不要。

### Frontend の変更

`createRealtimeToken` で取得した ephemeral key を使い WebRTC で接続する処理が新規に必要。

1. `POST /api/realtime/token` で `ek_...` を取得
2. `RTCPeerConnection` を生成し、マイク音声を `addTrack`
3. `ontrack` で受信音声を `<audio>` に接続
4. SDP offer を `POST https://api.openai.com/v1/realtime/calls?model=...` へ、`Authorization: Bearer ek_...` で送信
5. 返却された SDP answer を `setRemoteDescription`
6. データチャネルで `conversation.item.input_audio_transcription.completed` を受け取り、文字起こしを Backend へ保存

面接画面は Turn 経路と排他にする。PRD 6.2 のとおり両立はさせない。

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

`realtime_usage_logs` の削除は不可逆なので、**先にコード側の参照を落とし、1リリース以上空けてから down SQL 付きでテーブルを落とす**。完了レコードが 0 件のためデータ損失はないが、順序は守る。

同時接続制御と月次アラートは Realtime 専用の実装なので、Turn 経路に同等機能が要るかは別途判断する。**この判断を保留したまま削除すると、コスト暴走への防御が無くなる点に注意。**

## 共通: Turn 経路のコスト計測

採用案に関わらず必要。既存の `realtime_usage_logs` を経路非依存の形へ一般化する。

```
interview_usage_logs
  id, user_id, interview_session_id
  route            'turn' | 'realtime'
  duration_seconds
  stt_seconds, stt_cost_usd
  llm_input_tokens, llm_output_tokens, llm_cost_usd
  tts_characters, tts_cost_usd
  cost_usd         合計
  status, started_at, ended_at
```

単価はすべて環境変数で上書き可能とし、既定値を現行価格に合わせる。

| 環境変数 | 現行の既定 | 修正後 |
| --- | ---: | ---: |
| `REALTIME_AUDIO_INPUT_COST_PER_1M_USD` | 100.0 | 32.0 |
| `REALTIME_AUDIO_OUTPUT_COST_PER_1M_USD` | 200.0 | 64.0 |
| `REALTIME_CACHED_AUDIO_INPUT_COST_PER_1M_USD` | 20.0 | 0.40 |
| `STT_COST_PER_MIN_USD` | （新規） | 0.006 |
| `TTS_COST_PER_1M_CHARS_USD` | （新規） | 15.0 |

`gpt-realtime-mini` を採用する場合は音声単価が $10 / $20 になるため、モデルと単価の対応をコードに埋めず、環境変数で揃える運用とする。

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
| Frontend（案 B） | Playwright。面接開始で ephemeral key 取得 → WebRTC 接続確立 → 音声トラック受信までを検証。OpenAI へは接続せずモックする |
| 回帰 | Turn 経路の既存テストが通ること |

案 B の応答遅延（PRD N-1）は自動テストで測れないため、**staging での実測を受け入れ条件とする**。

## 未検証

- `gpt-realtime-mini` の日本語発話品質。実際に聞いて判断する必要がある
- WebRTC 接続の確立時間。N-1 の 1.0 秒に含めるかは実装後に判断する
- `gpt-4o-mini-transcribe` へ下げた場合の文字起こし精度。レポート採点の入力になるため、精度低下がスコアに影響しないかの確認が要る
