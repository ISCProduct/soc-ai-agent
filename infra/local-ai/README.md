# local-ai — OpenAI 障害時の縮退先（推論 PC）

親 Issue [#1831](https://github.com/ISCProduct/soc-ai-agent/issues/1831) / Backlog [SOCAIAGENT-464](https://iwasaki-ac.backlog.com/view/SOCAIAGENT-464) の子A（[#1832](https://github.com/ISCProduct/soc-ai-agent/issues/1832) / [SOCAIAGENT-465](https://iwasaki-ac.backlog.com/view/SOCAIAGENT-465)）。

Ubuntu 8GB・GPU なしの推論 PC 向けに、音声面接の縮退先を独立 Compose で立てる。

| 工程 | コンテナ | Backend が叩く契約 |
|---|---|---|
| STT | Whisper.cpp server | `POST /inference`（multipart、`language=ja`） |
| LLM | Ollama | `POST /v1/chat/completions` |
| TTS | VOICEVOX engine（CPU） | `POST /audio_query` → `POST /synthesis`（WAV） |

Backend 側の縮退ロジックは `Backend/internal/aifailover`（子B #1833）。接続先が未設定なら縮退しない。詳細は [`docs/wiki/ai-failover.md`](../../docs/wiki/ai-failover.md)。

本体の `docker compose`（db / app / frontend 等）とは**別**。このディレクトリだけで完結する。

## 前提

- Docker / Docker Compose v2
- 推論 PC の API は**認証なし・平文**。既定の公開先は `127.0.0.1`（`LOCAL_AI_BIND`）
- Backend から到達できること。**本番 Backend は Fargate のため、推論 PC の LAN IP だけでは届かない**。VPN・SSM ポートフォワード・PrivateLink 等が必要
- 推奨スペック: メモリ 8GB 以上・GPU なし（本 README の数値は**実機未検証**）

## セットアップ

```bash
cd infra/local-ai
cp .env.example .env   # 初回。setup.sh でもコピーする
# VPN 越しに公開するときだけ LOCAL_AI_BIND をその側のアドレスに変更（0.0.0.0 は避ける）
./scripts/setup.sh     # build / up / ollama-init（pull + warm-up）
./scripts/healthcheck.sh
./scripts/smoke.sh
./scripts/bench.sh     # 任意。メモリと単発レイテンシ（LLM は1回目/2回目）
```

停止:

```bash
docker compose down
# モデル volume も消すとき（次回 setup で ollama-init が再 pull）
docker compose down -v
```

## Backend への接続

推論 PC へ Backend から到達できるホスト名/IP を入れて、Backend の環境変数（または Terraform の `ai_failover_local_*`）へ渡す。

```bash
AI_FAILOVER_LOCAL_STT_URL=http://10.0.0.5:8080
AI_FAILOVER_LOCAL_LLM_URL=http://10.0.0.5:11434/v1
AI_FAILOVER_LOCAL_LLM_MODEL=qwen3:1.7b
AI_FAILOVER_LOCAL_TTS_URL=http://10.0.0.5:50021
AI_FAILOVER_LOCAL_TTS_SPEAKER=1
```

staging の稼働中インスタンスは user_data だけでは届かないことがある。`/opt/app/.env` の手更新が必要な場合は `docs/wiki/ai-failover.md` を参照。

## モデルとメモリ（目安・実機未検証）

| コンポーネント | 既定 | 備考 |
|---|---|---|
| Whisper | `tiny` | `.env` の `WHISPER_MODEL=base` で精度寄り（メモリ増） |
| Ollama | `qwen3:1.7b` | 8GB ではギリギリの想定。OOM したら `qwen3:0.6b`。`OLLAMA_KEEP_ALIVE=-1` と warm-up でコールド回避 |
| VOICEVOX | CPU イメージ（タグ固定） | 話者 ID は `VOICEVOX_SPEAKER` / Backend の `AI_FAILOVER_LOCAL_TTS_SPEAKER` |

同時起動時のホスト全体 8GB 以内に収まるかは**未実測**。`./scripts/bench.sh` の `free -h` と `docker stats` を推論 PC で取り、下記へ追記する。

### 実機未検証（明示）

次はリポジトリ整備時点で未確認。推論 PC で測ったら Issue / 本 README を更新する。

- [ ] Ubuntu 8GB・GPU なしでの同時起動（Whisper + Ollama + VOICEVOX）
- [ ] 起動後の常駐メモリが 8GB に収まること
- [ ] STT / LLM（1回目・2回目）/ TTS の単発レイテンシ（`bench.sh`）
- [ ] 日本語面接音声での Whisper `tiny` / `base` の聞き取り品質
- [ ] webm（`--convert`）経路の通し確認
- [ ] Ollama `qwen3:1.7b` と `0.6b` の面接官応答の品質差
- [ ] VOICEVOX 話者 ID `1` の声質が面接向けとして許容できるか
- [ ] Backend（Fargate / staging）から VPN 等経由の通し試験（OpenAI を意図的に落としての縮退）

## ディレクトリ

```text
infra/local-ai/
  docker-compose.yml
  .env.example
  .gitignore
  README.md
  whisper/
    Dockerfile
    entrypoint.sh
  scripts/
    setup.sh
    healthcheck.sh
    smoke.sh
    bench.sh
    common.sh
  testdata/smoke.wav
```

## トラブル

| 症状 | 確認 |
|---|---|
| `healthcheck` で ollama モデル NG | `compose logs ollama-init`。`down -v` 後は init が再 pull する |
| `smoke` の LLM が `<think>` 除去後空 | `max_tokens` 不足やモデル不調。Backend でも失敗する |
| Whisper が起動ループ | モデル取得失敗の残骸。volume を消すか不完全 `ggml-*.bin` を削除 |
| Whisper が webm で失敗 | entrypoint は `--convert` 付き。ffmpeg がイメージに含まれる公式タグか |
| メモリ不足で Ollama が落ちる | `.env` で `OLLAMA_MODEL=qwen3:0.6b` にして setup し直す |
| Backend から繋がらない | `LOCAL_AI_BIND` と VPN/トンネル。Fargate から LAN IP 直指定は不可。Docker の ports は ufw を迂回する |

## 関連

- 縮退の動き: [`docs/wiki/ai-failover.md`](../../docs/wiki/ai-failover.md)
- Backend クライアント: `Backend/internal/aifailover/local.go`
