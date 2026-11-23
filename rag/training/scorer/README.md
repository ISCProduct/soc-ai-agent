# 軽量採点モデルの学習（#1834 の4・#2170）

書き出しジョブ（Backend の `scorerexport`、#2168）が作った JSONL から、面接の採点モデルを学習します。
モデルは文字 n-gram のハッシュ特徴量と、ルーブリックの軸ごとの線形の順序回帰です。
推論は Go（#2171）がアプリのサーバーの中で行うので、**特徴量の手順とモデルファイルの形式は、Go と完全に同じにします**。

> **LLM ラベルで学習します。** ラベルは LLM の採点（`interview_reports.scores_json`）で、教員の採点ではありません。
> LLM の採点を学習ラベルに使うのは、蒸留禁止の方針の例外として #1834 に限って許容されたもので、他の学習用途には使いません。
> 学習の最後に出す MAE・QWK は「LLM ラベルとの一致」で、教員の評価セットでの精度ではありません（実運用の精度は #2172）。

## 使い方

`rag/` で実行します。依存は `rag/requirements.txt` の範囲（numpy）だけです。

```sh
cd rag
python -m training.scorer \
  --input  <書き出し1回ぶんのディレクトリ（S3 の <prefix>/runs/<実行ID>/ を落としたもの）> \
  --model-out   scorer.bin \
  --report-out  scorer-report.md
```

| 引数 | 既定 | 意味 |
| --- | --- | --- |
| `--seed` | 20261117 | 利用者単位の分割の乱数の種 |
| `--eval-fraction` | 0.2 | 評価用に回す利用者の割合（少なくとも1人、多くとも 人数-1 人） |
| `--epochs` / `--learning-rate` / `--l2` | 300 / 0.2 / 1e-5 | 勾配法（全件の勾配で Adam）の設定 |

入力の誤り（マニフェストが無い・形が違う・使える行が無い・利用者が1人だけ）は、終了コード 2 で止まります。

## 入力

`manifest.json` がある回（書き出しを終えた回）だけを読み、マニフェストの `files` に載ったファイルだけを読みます。

- マニフェストの `format_version` が 1、`label_source` が `"llm"` であること。軸の一覧は `rubric_keys` から取ります。
- 1行（面接1回）は `scorerexport.Sample` の形です。次の行は学習に入れず、理由ごとに数えてレポートに出します
  （ログには行の中身を書きません）。
  - JSON でない（`invalid_json`）、`format_version` が 1 でない（`unsupported_format_version`）
  - `label_source` が `"llm"` でない（`unexpected_label_source`。教員の採点は評価用なので学習に混ぜない）
  - `subject`・`sample_id` が無い（`missing_id`）、`sample_id` の重複（`duplicate_sample_id`）
  - ラベルが Go の `ValidateRubricScores` と同じ条件を満たさない（軸の過不足・整数でない・0〜5 の外。`invalid_labels`）
  - `turns` が空か形が違う（`invalid_turns`）
  - 不正な UTF-8 のバイトを含む（`invalid_utf8`。置き換えて読むと、改変された回答や壊れた `subject` を学習してしまうため）
- マニフェストの `run_id` は空でない文字列であること（モデルのヘッダとレポートに書き、モデルを元の書き出しまでたどるため）。
- `--model-out` と `--report-out` に同じパスは指定できません。

## 特徴量の手順

Go で同じ実装をするときは、この順に行います（`features.py`）。

1. **正規化**（質問・回答の1つずつに）
   1. 対になっていないサロゲート（U+D800〜U+DFFF）を U+FFFD にする（Go の `encoding/json` の読み方に合わせる）。
   2. NFKC（Go は `golang.org/x/text/unicode/norm` の `norm.NFKC.String`）。
   3. ASCII の `A`〜`Z` だけを小文字にする（`strings.ToLower` は使わない。「İ」などで Python と結果が違うため）。
   4. 次の空白文字の連続を半角スペース1つにし、前後の空白を落とす。集合は Go の `unicode.IsSpace` と同じ：
      U+0009〜U+000D、U+0020、U+0085、U+00A0、U+1680、U+2000〜U+200A、U+2028、U+2029、U+202F、U+205F、U+3000。
2. **n-gram**：正規化した文字列の Unicode コードポイント（Go の `[]rune`）で、1〜3文字の n-gram をすべて取る。
   質問と回答の1つずつの中で作り、ターンをまたいだ n-gram は作らない。
3. **ハッシュ**：`名前空間 + ":" + n-gram` の UTF-8 を FNV-1a 32bit（Go の `hash/fnv` の `New32a`）にかけ、
   下位 16 ビット（`h & 0xFFFF`）を添字にする（次元 2^16 = 65536）。名前空間は質問が `q`、回答が `a`。
4. **値**：面接1回ぶんの全ターンで添字ごとの出現回数 c を数え、値を `1 + ln(c)` にする。
   最後に、添字の昇順に float64 で二乗和を足してノルムを出し、各値をノルムで割る（L2 正規化）。特徴が無ければ空。

### ゴールデンベクタ（Go との一致の確認）

[`rag/testdata/scorer_feature_golden.json`](../../testdata/scorer_feature_golden.json) に、
FNV-1a の既知の値・正規化の結果・特徴の添字・面接1回ぶんの特徴量（`[添字, 出現回数, 値]`）を書いています。

- Python は `tests/test_scorer_features.py` が、この実装と一致することを確かめます。
- Go の推論（#2171）も同じファイルを読み、`counts` は完全一致、`values` は誤差 1e-12 以内で一致することを確かめます
  （既存の `rag/testdata/char_limit_cases.json` と同じ置き方です）。
- 手順を変えたら `FEATURE_SPEC_VERSION` を上げ、`python -m training.scorer.golden --write` で作り直します。
- NFKC の結果は Unicode の版で変わりうるため、ゴールデンには版によって変わらない文字だけを入れています。

### Unicode の版をそろえる

NFKC の結果は Unicode の版で変わります（例：U+A7F2 は Unicode 13 ではそのまま、14 以降は `C`）。
版は Python の版で決まり（3.10 は 13.0.0、3.12 は 15.0.0）、Go の `golang.org/x/text/unicode/norm` も
Go の版で変わります（今の x/text v0.43.0 では Go 1.26 までが 15.0.0、Go 1.27 からが 17.0.0）。

- 学習したときの版（`unicodedata.unidata_version`）を、ヘッダの `features.unicode_version` に書きます。
- Go の推論は `norm.Version` と一致しなければモデルを使いません。Python の `model.load` も、実行環境の版と違えばエラーにします。
- **学習は、Go の版に合う Python で行います。** 今のバックエンド（Go 1.26）は 15.0.0 なので、
  RAG のイメージと同じ Python 3.12 で学習します（CI のテストは 3.10 で動きますが、テストで作るモデルはその場で読むだけです）。
  Go を 1.27 以降に上げるときは、学習に使う Python の版も合わせて見直します。
- 対になっていないサロゲートは、Go では JSON を読む時点で U+FFFD になり同じ入力を作れないので、ゴールデンには入れず、
  Python のテスト（`test_normalize`）だけで確かめています。

## 学習

- 分割は **利用者（`subject`）単位** です。`subject` を整列してから種を固定した乱数で並べ替え、
  先頭の `round(人数 × eval_fraction)` 人を評価用にします。同じ人の面接は必ず同じ側に入ります。
- 軸ごとに、all-threshold のロジスティックの順序回帰を学習します。
  閾値 θ_k（k = 0〜4）ごとに「スコアが k より大きいか」を σ(w·x − θ_k) で当てる2値の対数損失を足し、
  重みに L2 正則化をかけます。重みは 0、閾値はラベルの分布から決まる値から始めます。
- 評価は、保存したファイルを読み直したモデル（float16 の重み）で行います。Go が読むのと同じ中身で測るためです。

## モデルファイルの形式

```
先頭 8 バイト  "SOCSCORE"（ASCII）
4 バイト       ヘッダの長さ N（uint32、リトルエンディアン）
N バイト       ヘッダ（JSON、UTF-8）
残り           重み（float16、リトルエンディアン）
```

- **重みの並び**：`rubric.keys` の順に軸ごとに 65536 個ずつ、添字の昇順（`weights[axis * 65536 + index]`）。
  大きさは 5 軸 × 65536 × 2 バイト = 655,360 バイトで、ファイル全体は 2.7MB（2,700,000 バイト）以内です
  （超えるときは保存せずにエラーにします）。
- **ヘッダ**の主な項目：

  | 項目 | 内容 |
  | --- | --- |
  | `format` / `format_version` | `"soc-interview-scorer"` / `1` |
  | `rubric.version` | ルーブリックの版。`strings.Join(RubricKeys(), ",") + "\|0-5"`（例：`logic,specificity,ownership,communication,enthusiasm\|0-5`）の UTF-8 の SHA-256 の先頭16桁に `sha256:` を付けたもの |
  | `rubric.keys` / `score_min` / `score_max` | 軸の一覧（重みの並び順）と値域 |
  | `features` | 特徴量の設定（`spec_version`・`hash: "fnv1a32"`・`dim`・`ngram_min`/`ngram_max`・正規化の手順など）と、NFKC に使った Unicode の版 `unicode_version` |
  | `thresholds` | 軸ごとの閾値 5 つ（float64、昇順とは限らない） |
  | `weights` | `dtype: "float16"`・`byte_order: "little"`・`layout: "axis_major"`・`shape`・`byte_length`・`sha256`（重みのバイト列の SHA-256） |
  | `label_source` | `"llm"` |
  | `training` | 学習・評価の件数と人数、読み飛ばした行の数、書き出しの実行 ID、種、学習の設定 |

- **読み込み**（`model.load`）は、先頭のバイト列・形式の版・特徴量の設定・Unicode の版・重みの長さ・**重みの SHA-256** を確かめ、
  合わなければエラーにします。Go も同じ確認をしてから使います。
- **推論**：x を上の手順の特徴量として、軸ごとに `z = Σ value × float64(weight[axis][index])`（添字の昇順に足す）。
  スコアは `0 + (z > θ_k となる k の数)`、確信度の目安には期待値 `Σ σ(z − θ_k)` を使えます。

## Go での推論と配線確認

推論は `Backend/internal/services/interview/shadowscorer` が、このファイルと同じ確認をしてから採点します。
アプリでは `INTERVIEW_SHADOW_SCORER=true` と `INTERVIEW_SHADOW_MODEL_PATH` を置いたときだけ、面接レポートの保存後に裏で採点し、`interview_shadow_scores` へ保存します。発話は学習書き出しと同じ伏字を通してから特徴量にします。保存する `model_version` はモデルファイル全体の SHA-256 です。既定はオフです。採点を止めたあとも、保存済みの結果は管理者が読めます。結果は `interview_reports.scores_json`、`user_weight_scores`、マッチング、学生の画面には使いません。

人工データで「書き出し → 学習 → Go の推論」まで通す確認は `rag/tests/test_scorer_go_wiring.py` です。Python 3.12（Unicode 15.0.0）と Go が要ります。結果は配線が通ったことだけで、精度の主張には使いません。

## 評価レポート

先頭に「LLM ラベルで学習」「LLM ラベルとの一致であり、教員の評価セットでの精度ではない」と書き、
評価用の分割での軸ごとの MAE と QWK（二次の重み付きカッパ）を出します。
評価データのラベルと予測がどちらも1種類だけのときは、QWK は定義できないので N/A にします。
