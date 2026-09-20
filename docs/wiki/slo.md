# SLO とアラート (#1187)

本番の信頼性目標と、それを測る方法・鳴らすアラートを定義する。

## 前提: この環境は常時稼働していない

本番は**稼働日のみ起動**する（`prod-uptime-scheduler.yml` が SSM `/soc-app/prod-uptime-dates` を毎時照合して ECS と RDS を起動/停止する）。
そのため一般的な「30日間の稼働率」型の SLO はそのままでは使えない。

**時間ベースではなくイベントベースで定義する。** `good events / valid events` で測り、
分母には**稼働日として登録された日のリクエストだけ**を含める。停止日を分母に入れると、
バジェットが毎日勝手に枯渇するか、逆に「停止＝イベント0」で SLO が常に満たされて意味を失う。

### 例外: 稼働日に起動していない場合

**稼働日に起動できていない時間帯は、可用性 0% として扱う。**
リクエストが届かないので自動計測上はイベント0になるが、それを「SLO 違反なし」と読むのは装飾でしかない。
利用者から見れば全断である。

この検知は #1354 / #1355 で入れた起動ジョブの失敗通知が担う。
「起動ジョブが失敗して誰も気づかない」が過去に成立していた構成だったため、SLO より先にそこを塞いである。

## SLI 定義

計測ソースは **ALB の CloudWatch メトリクス**。標準メトリクスなので追加コストがかからず、
アプリが停止していても欠損として区別できる。Prometheus は常設していない（理由は `operations.md` 3.2 節）。

| # | SLI | good events | valid events | データ元 |
|---|---|---|---|---|
| 1 | API 可用性 | `RequestCount - (HTTPCode_Target_5XX_Count + HTTPCode_ELB_5XX_Count)` | `RequestCount` | TargetGroup `soc-app-be-tg` |
| 2 | 画面可用性 | 同上 | `RequestCount` | TargetGroup `soc-app-fe-tg` |
| 3 | API レイテンシ | `TargetResponseTime` p95 が閾値以下だった期間 | 稼働中の全期間 | TargetGroup `soc-app-be-tg` |
| 4 | 稼働日の起動成功 | 稼働日のうち、予定時刻までに `/health` が 200 を返した日数 | 稼働日数 | 起動ジョブの結果 |

### レイテンシ SLI の限界（先に書いておく）

ALB のメトリクスは**パス別に割れない**。チャット・面接・履歴書レビューは LLM 呼び出しを含むため
数秒〜数十秒かかるのが正常で、それが同じ `TargetResponseTime` に混ざる。
つまり「p95 が 5 秒」という数字は、AI 経路が多い日ほど悪化するが、それは異常ではない。

**フロー別のレイテンシを見たいときは `/metrics` を手元からスクレイプする**（`operations.md` 3.2 節）。
`backend_request_duration_seconds` は `url` ラベルでルート別に割れる。

常時フロー別に見る必要が出たら、そのとき初めて常設の Prometheus を検討する。

## 目標値

**100% を目標にしない。** 目標は「利用者が不満を持ち始める点」から逆算する。

| SLI | 目標 | 根拠 |
|---|---|---|
| 1. API 可用性 | 稼働日のリクエストの **99.5%** が 5xx でないこと | 展示会で1日3,000リクエストなら、5xx は15件まで許容。学生が1回触って失敗する確率を 0.5% 未満に抑える |
| 2. 画面可用性 | 同 **99.5%** | 同上 |
| 3. API レイテンシ | `TargetResponseTime` p95 が **10秒以下**の時間が 95% | AI 経路込みの値。非AI経路だけなら1秒台であるべきだが、ALBでは分離できないため緩い値を置く |
| 4. 稼働日の起動成功 | **100%**（1回でも失敗したら要ポストモーテム） | 展示会当日に起動しない = その日の価値がゼロ。バジェットを持たせる性質の指標ではない |

SLI 4 だけ 100% なのは、母数が年に数日しかなく「1回の失敗＝1日の全損」になるため。
バジェットで許容する種類の失敗ではない。

## エラーバジェットとアラート

### 規模に合わせて教科書から外す

Google SRE の多重ウィンドウ・バーンレートアラート（1h+5m で 14.4x など）は、
**トラフィックが十分ある前提**で成り立つ。本環境は稼働日でも1日数千リクエスト規模で、
5分ウィンドウに数十リクエストしか入らない。この量でバーンレートを計算すると、
5xx が 1〜2 件出ただけで「14.4x」に達して鳴り続ける。

そのため、当面は**絶対数のしきい値**で鳴らす。

| アラート | 条件 | 対応 |
|---|---|---|
| 5xx 急増 | 5分間で `HTTPCode_Target_5XX_Count + HTTPCode_ELB_5XX_Count` が **10件以上** | 即対応。直近のデプロイをロールバック候補にする |
| ターゲット全損 | `HealthyHostCount` が 5分間 **0** | 即対応。タスクのクラッシュループを疑う |
| レイテンシ悪化 | `TargetResponseTime` p95 が15分間 **10秒超** | 調査。LLM 側の遅延か、タスク枯渇かを切り分ける |
| バジェット消費 | 稼働日ごとに日次で確認（自動アラートにしない） | 週次で傾向を見る |

トラフィックが 1日1万リクエストを超えるようになったら、バーンレート方式へ移行する。

### 欠損データの扱い

**停止日にアラートを鳴らしてはいけない。** CloudWatch アラームの `treat_missing_data` は
`notBreaching` にする。停止中はメトリクス自体が届かないため、`breaching` にすると停止日に鳴り続ける。

なお `HTTPCode_Target_5XX_Count` は、5xx が一度も発生していないターゲットグループには
**メトリクス自体が存在しない**（`aws cloudwatch list-metrics` に現れない）。
アラーム作成は可能だが、この点でも `notBreaching` が必要になる。

### 通知先

#1355 で入れた Discord 通知経路に合流させる。CloudWatch アラーム → SNS → Lambda → Discord。
Discord Lambda は既にリポジトリにある（`Backend/cmd/discord-lambda`）。

**未実装**: CloudWatch アラームと SNS の Terraform 定義。本番の `terraform apply` は #1360 の
ドリフトが解消されるまで実行できないため、コード追加もそのあとに行う。

## エラーバジェットポリシー

結果が伴わない SLO は装飾でしかない。枯渇したときに何を止めるかを決めておく。

1. **バジェットを 50% 消費**したら、原因の調査をその週の作業に入れる
2. **バジェットを使い切ったら**、次の稼働日までの間、新機能の追加をやめて信頼性作業を優先する
3. **展示会の1週間前からは**、バジェット残量にかかわらず本番へ影響する変更を入れない（`release` ブランチで止める）
4. SLI 4（起動成功）が1回でも失敗したら、ポストモーテムを書く。TTD と TTM を数値化し、検知時間を縮める施策を必ず1つ入れる

## 計測のしかた

```bash
LB=app/soc-app-alb/44dbd832d9a69de2
TG=targetgroup/soc-app-be-tg/8977ea6008ddf41d

# 稼働日の総リクエスト数（valid events）
aws cloudwatch get-metric-statistics \
  --namespace AWS/ApplicationELB --metric-name RequestCount \
  --dimensions Name=TargetGroup,Value=$TG Name=LoadBalancer,Value=$LB \
  --start-time 2026-09-20T00:00:00Z --end-time 2026-09-21T00:00:00Z \
  --period 86400 --statistics Sum

# 5xx（bad events）
aws cloudwatch get-metric-statistics \
  --namespace AWS/ApplicationELB --metric-name HTTPCode_Target_5XX_Count \
  --dimensions Name=TargetGroup,Value=$TG Name=LoadBalancer,Value=$LB \
  --start-time 2026-09-20T00:00:00Z --end-time 2026-09-21T00:00:00Z \
  --period 86400 --statistics Sum

# レイテンシ p95
aws cloudwatch get-metric-statistics \
  --namespace AWS/ApplicationELB --metric-name TargetResponseTime \
  --dimensions Name=TargetGroup,Value=$TG Name=LoadBalancer,Value=$LB \
  --start-time 2026-09-20T00:00:00Z --end-time 2026-09-21T00:00:00Z \
  --period 3600 --extended-statistics p95
```

データが返らない日は「停止日」か「稼働日だが起動に失敗した日」のどちらか。
SSM `/soc-app/prod-uptime-dates` と突き合わせて区別する。

```bash
aws ssm get-parameter --name /soc-app/prod-uptime-dates --query Parameter.Value --output text
```

## 見直す条件

- 1日のリクエストが1万を超えたら、バーンレートアラートへ移行する
- 常時稼働へ切り替えたら、イベントベースから時間ベースの SLO も併用する
- フロー別のレイテンシを常時見る必要が出たら、常設の Prometheus を検討する（コストとの兼ね合い）
