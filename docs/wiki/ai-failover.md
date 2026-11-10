# OpenAI 障害時の音声面接の縮退（#1833）

OpenAI の障害時に、音声面接（`/start-turn`・`/turn`）の STT・LLM・TTS を、工程ごとにローカルの推論先へ切り替える。
親 #1831（OpenAI 障害時にローカル AI へ縮退する音声面接の基盤）の子B。ローカルの推論先は子A（#1832）で用意する。

既存の `Backend/internal/openai/fallback.go`（ローカル → OpenAI）とは逆向きの、別の仕組み（`Backend/internal/aifailover`）。

## 動き

| 工程 | 通常 | 縮退先 |
|---|---|---|
| STT | OpenAI の文字起こし | Whisper.cpp の server（`POST /inference`） |
| LLM（面接官の応答） | OpenAI の chat | OpenAI 互換の API（Ollama の `/v1/chat/completions`）。Qwen3 の `<think>` は落とす |
| TTS | OpenAI の TTS（mp3） | VOICEVOX エンジン（`/audio_query` → `/synthesis`、WAV） |

- **縮退するのは一時的な障害だけ**: 429・5xx・タイムアウト・接続できない（名前解決・接続拒否）。認証（401/403）・入力不正（400/422 など）は隠さずにそのまま返す。クライアントが切断したターンも縮退しない
- **工程ごとに独立**: STT だけ落ちていれば STT だけローカルを使う
- **Circuit Breaker**: 工程ごとに、縮退した失敗が続けて `AI_FAILOVER_BREAKER_THRESHOLD` 回（既定 3）起きたら、`AI_FAILOVER_BREAKER_COOLDOWN_SECONDS`（既定 30）秒の間 OpenAI を呼ばずにローカルへ回す。その後1回だけ OpenAI を試し、成功すれば戻す。障害中に毎回 OpenAI のタイムアウトを待たないため
- **音声の形式**: ローカルの TTS は WAV を返す。応答の音声の Content-Type は中身から決める（WAV なら `audio/wav`、それ以外は `audio/mpeg`）。ブラウザはどちらもそのまま再生できる
- **応答のメタ**: ローカルで処理した工程を `ai_fallback`（例 `["stt","llm","tts"]`）で返す。OpenAI だけで済んだら空
- **利用量**: ローカルで処理した STT・TTS は OpenAI の利用量として記録しない。STT をローカルで処理したターンは、OpenAI の高精度モデルへの再送もしない
- **ローカルも失敗したら**: 今までと同じく、STT は「聞き取れませんでした」、LLM は言い換えの応答、TTS は音声なし（テキストのみ）で面接を続ける

## 設定

接続先が未設定の工程は縮退しない（今までと同じ）。

| 環境変数 | 既定 | 内容 |
|---|---|---|
| `AI_FAILOVER_LOCAL_STT_URL` | （なし） | Whisper.cpp の server（例 `http://10.0.0.5:8080`） |
| `AI_FAILOVER_LOCAL_LLM_URL` | （なし） | OpenAI 互換の API（例 `http://10.0.0.5:11434/v1`） |
| `AI_FAILOVER_LOCAL_LLM_MODEL` | `qwen3:1.7b` | ローカルの LLM のモデル |
| `AI_FAILOVER_LOCAL_TTS_URL` | （なし） | VOICEVOX エンジン（例 `http://10.0.0.5:50021`） |
| `AI_FAILOVER_LOCAL_TTS_SPEAKER` | `1` | VOICEVOX の話者 ID |
| `AI_FAILOVER_BREAKER_THRESHOLD` | `3` | 続けて失敗したら開く回数 |
| `AI_FAILOVER_BREAKER_COOLDOWN_SECONDS` | `30` | 開いている時間 |
| `AI_FAILOVER_LOCAL_TIMEOUT_SECONDS` | `60` | ローカルの推論先の待ち時間（GPU なしの推論 PC は遅いので長め） |

## 未対応（別の PR）

- 全音声失敗時のテキストでの面接継続の画面（フロント）。`ai_fallback` と音声なしの応答を使う
- Realtime 失敗時の、確定した履歴からの Turn 方式への移行（今の画面は Realtime の経路を使っていない）
- 実機（Ubuntu 8GB・GPU なし）での応答時間・品質の確認（子A #1832 の環境が要る）
