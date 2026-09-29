# AI面接 音声評価フィクスチャ

このディレクトリは、AI面接の音声認識を再現可能に評価するためのR&D用データです。

- `manifest.jsonl`: ケース、正解テキスト、想定する誤り、評価観点（**追跡対象外**）
- `audio/*.wav`: 16kHz・monoの基準音声（**追跡対象外**）
- `audio/*.webm`: 現行Backendへ送信する形式に近い音声（**追跡対象外**）
- `make_degraded.py`: 上の基準音声から劣化版を作る生成器（追跡対象）
- `manifest_degraded.jsonl`: 生成器が書き出す劣化版のマニフェスト（追跡対象）

音声とレベル実測値（`degraded_levels.tsv`）は追跡しません。8ケース×27条件×2形式で
448ファイル・約115MBになり、リポジトリを圧迫するためです。
**作り方（生成器とマニフェスト）だけを追跡し、音声は各自の手元で作り直します。**

合成音声は発音・雑音・個人差を十分に再現しないため、モデルの最終判断には使用しません。まず録音経路やAPI連携の回帰テストに利用し、次に同意を得た実発話を追加してください。

## 比較手順

1. manifestの`reference_text`を正解として保存する
2. 同じ音声を`gpt-4o-transcribe`と`gpt-4o-mini-transcribe`に送る
3. CER、固有名詞、数値、意味変化、応答時間を記録する
4. 文字起こし結果を面接レポートへ入力し、評価結果の差を比較する

## 劣化音声を作る（#1603 ①実測）

「クライアントの環境が悪いときに誤変換が起きるか」を測るためのデータです。
`condition` は `noisy` / `reverb` / `quiet` / `clipped` / `lowband` / `lowbitrate` /
`dropout` / `fast` と、その組み合わせ（`combo-*`）。強度は条件名に入ります。

前提: `ffmpeg`（`anoisesrc` / `aecho` / `libopus` を使う）と、
上記の**追跡対象外の基準音声**（`manifest.jsonl` と `audio/*.wav`）が手元にあること。

```sh
cd docs/research/interview-audio-eval
python3 make_degraded.py                  # 224ケース（8×28条件）を生成。約1分
python3 make_degraded.py --only noisy     # condition の前方一致で絞る
python3 make_degraded.py --manifest-only  # 音声を作らずマニフェストだけ更新
```

生成されるもの:

- `audio/degraded/<ケース>__<condition>.wav` / `.webm`
- `manifest_degraded.jsonl`（`reference_text` は元ケースと同一。劣化しても話した内容は変わらない）
- `degraded_levels.tsv`（実測した mean/max 音量・到達SNR・クリップ率・長さ）

**強度は実測して合わせています。** 雑音は「音声のRMSを測る → 目標SNRになる雑音ゲインを
計算する → 生成した雑音を測り直す」の3段で決めます。`amix` の重みだけではSNRは決まりません。
雑音の `seed` はケースと条件から決まるので、同じ入力からは毎回同じ音声ができます。

## 評価の実行

```sh
cd Backend
# 劣化音声（WAV）。補助語なし
go run ./cmd/sttbench -manifest ../docs/research/interview-audio-eval/manifest_degraded.jsonl \
  -format wav -models gpt-4o-transcribe,gpt-4o-mini-transcribe -out <リポジトリ外>

# 本番が送る形式（WEBM）
go run ./cmd/sttbench -manifest ../docs/research/interview-audio-eval/manifest_degraded.jsonl \
  -format webm -out <リポジトリ外>

# 補助語あり（BuildSTTHints を通す。`会社名|読み|職種|企業情報`）
go run ./cmd/sttbench -manifest ../docs/research/interview-audio-eval/manifest_degraded.jsonl \
  -hints '株式会社サンプルソフト|サンプルソフト|バックエンドエンジニア|Go と AWS を使った SaaS のバックエンド開発。Docker と MySQL、TypeScript も利用。' \
  -out <リポジトリ外>

# 1条件だけ
go run ./cmd/sttbench -manifest ../docs/research/interview-audio-eval/manifest_degraded.jsonl \
  -condition noisy-white-snr0 -out <リポジトリ外>
```

`-out` は必ずリポジトリ外へ。認識本文が入るため、合成音声由来でも追跡しません。
**`mini` は同じ入力でも結果が安定しません。1回の試行で結論を出さないこと**
（`RESULTS_stt_hints.md` §4）。実測結果は `RESULTS_degraded.md`。

