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
- 推論 PC はアプリ（Backend）からプライベートネットワークで到達できること
- 推奨スペック: メモリ 8GB 以上・GPU なし（本 README の数値は**実機未検証**）

## セットアップ

```bash
cd infra/local-ai
cp .env.example .env   # 初回。setup.sh でもコピーする
./scripts/setup.sh     # build / up / ollama pull
./scripts/healthcheck.sh
./scripts/smoke.sh
./scripts/bench.sh     # 任意。メモリと単発レイテンシ
```

停止:

```bash
docker compose down
# モデル volume も消すとき
docker compose down -v
```

## Backend への接続

推論 PC の LAN IP を入れて、Backend の環境変数（または Terraform の `ai_failover_local_*`）へ渡す。

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
| Ollama | `qwen3:1.7b` | 8GB ではギリギリの想定。OOM したら `qwen3:0.6b` |
| VOICEVOX | CPU イメージ | 話者 ID は `VOICEVOX_SPEAKER` / Backend の `AI_FAILOVER_LOCAL_TTS_SPEAKER` |

同時起動時のホスト全体 8GB 以内に収まるかは**未実測**。`./scripts/bench.sh` の `free -h` と `docker stats` を推論 PC で取り、下記へ追記する。

### 実機未検証（明示）

次はリポジトリ整備時点で未確認。推論 PC で測ったら Issue / 本 README を更新する。

- [ ] Ubuntu 8GB・GPU なしでの同時起動（Whisper + Ollama + VOICEVOX）
- [ ] 起動後の常駐メモリが 8GB に収まること
- [ ] STT / LLM / TTS の単発レイテンシ（`bench.sh` の数値）
- [ ] 日本語面接音声での Whisper `tiny` / `base` の聞き取り品質
- [ ] Ollama `qwen3:1.7b` と `0.6b` の面接官応答の品質差
- [ ] VOICEVOX 話者 ID `1` の声質が面接向けとして許容できるか
- [ ] Backend（別マシン）からのプライベートネットワーク経由の通し試験（OpenAI を意図的に落としての縮退）

## ディレクトリ

```text
infra/local-ai/
  docker-compose.yml   # whisper / ollama / voicevox
  .env.example
  whisper/             # Whisper.cpp server の entrypoint
  scripts/
    setup.sh
    healthcheck.sh
    smoke.sh
    bench.sh
  testdata/smoke.wav   # smoke / bench 用の短い WAV
```

## トラブル

| 症状 | 確認 |
|---|---|
| `healthcheck` で ollama が NG | `docker compose logs ollama`。モデル pull 前でも `/api/tags` は 200 になる想定 |
| `smoke` の LLM が 404 | `OLLAMA_MODEL` を pull したか（`setup.sh`）。名前は `ollama list` で確認 |
| Whisper が webm で失敗 | entrypoint は `--convert` 付き。ffmpeg がイメージに含まれる公式タグを使っているか |
| メモリ不足で Ollama が落ちる | `.env` で `OLLAMA_MODEL=qwen3:0.6b` にして `./scripts/setup.sh` の pull を再実行 |
| Backend から繋がらない | 推論 PC の firewall / bind。Compose は `0.0.0.0` で publish している |

## 関連

- 縮退の動き: [`docs/wiki/ai-failover.md`](../../docs/wiki/ai-failover.md)
- Backend クライアント: `Backend/internal/aifailover/local.go`
