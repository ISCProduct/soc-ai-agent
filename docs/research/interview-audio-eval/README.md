# AI面接 音声評価フィクスチャ

このディレクトリは、AI面接の音声認識を再現可能に評価するためのR&D用データです。

- `manifest.jsonl`: ケース、正解テキスト、想定する誤り、評価観点（**未コミット**）
- `audio/*.wav`: 16kHz・monoの基準音声（**追跡対象外**）
- `audio/*.webm`: 現行Backendへ送信する形式に近い音声（**追跡対象外**）
- `make_degraded.py`: 上の基準音声から劣化版を作る生成器（追跡対象）
- `manifest_degraded.jsonl`: 生成器が書き出す劣化版のマニフェスト（追跡対象）
- `compare_degraded.py`: 結果JSONから「clean からの悪化」を数える集計（追跡対象）

音声とレベル値（`degraded_levels.tsv`）は追跡しません。8ケース×27条件×2形式で
448ファイル・約115MBになり、リポジトリを圧迫するためです。
**作り方（生成器・マニフェスト・集計）を追跡し、音声は各自の手元で作り直します。**

`.gitignore` の許可は `*.py` / `*.jsonl` の拡張子単位です。ファイル名の完全一致に
すると、生成器を増やす・マニフェストを改名した瞬間に無言で無視されるためです。
この副作用で基準の `manifest.jsonl` も追跡可能になりますが、対応する音声が
追跡されていないのでコミットはしていません。
**実発話を扱う場合は `real/` 以下か `*real*.jsonl` に置いてください**
（PII が入りうるので許可から外しています）。

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
- `degraded_levels.tsv`（mean/max 音量・発話区間のRMS・雑音のRMS・目標SNR・クリップ率・長さ）

### SNR ラベルの扱い（誤読しやすいので先に書く）

雑音は「**発話区間**のRMSを測る → 目標SNRになる雑音ゲインを計算する → そのゲインで
雑音を作る」の順で混ぜます。`amix` の重みだけではSNRは決まりません。

**この手順は SNR を検証していません。** `volume` フィルタは指示どおりの線形ゲインを
掛けるので、生成後の雑音を測り直して目標との差を出しても必ず 0 になります
（恒等式であって測定ではない）。以前の版はその 0 を「誤差 0.000 dB」として
品質の根拠にしていましたが、確かめていたのは `volume` フィルタの動作だけでした。
いまは `src_speech_mean_db` と `noise_mean_db` を生で TSV に残すだけにしています。

信号レベルは `silenceremove` で無音を落としてから測ります。ファイル全体のRMSだと
前後・語間の無音ぶん過小評価になり、ラベルより実際は楽な条件になります
（無音率はケースごとに違うのでケース間でもラベルが揃わない）。

`combo-noisy10-lowband` は `amix` の**後**にバンドパスを掛けるので、配信ファイルの
SNR は目標の 10dB ではありません（`post_filter` 列で分かります）。

`quiet` × `noisy` のように減衰と雑音を併用する条件は**ガードで弾いています**。
雑音ゲインは加工前のレベルから計算しているので、`pre` を掛けるとラベルが丸ごと
ずれます。追加するなら `pre` 適用後を測り直す実装にしてください。

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

## 結果の集計

`-out` の JSON から headline（「劣化で悪化 N/216」など）を出します。

```sh
cd docs/research/interview-audio-eval
python3 compare_degraded.py <リポジトリ外>/off.json
python3 compare_degraded.py <リポジトリ外>/off.json --with <リポジトリ外>/on.json
python3 compare_degraded.py <リポジトリ外>/off.json --keyword-cases-only
python3 compare_degraded.py --selftest   # 判定ロジックの自己テスト
```

比較相手は**正解テキストではなく同じケースの `clean` 結果**です。正解と比べると
`30パーセント` → `30%` のような表記ゆれを劣化のせいに数えてしまいます
（最初の集計はこれで誤りました）。判定の定義はスクリプト冒頭に書いてあります。

**`mean_cer` 等を見るときは必ず `errored_cases` を一緒に見てください。**
API エラーは平均の母数から外れるので、残高切れで大半が落ちた run は
件数を見ないと「CER が良くなった」ように読めます。4xx が3件続くと
`sttbench` 側が打ち切って `aborted: true` を立てます。

