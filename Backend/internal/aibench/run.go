package aibench

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"Backend/internal/services/costs"
)

// Run は全ケースを runs 回ずつ実行して集計する。
//
// 実行順は「ケースごとに runs 回連続」ではなく「1周ずつ」にする。
// 連続実行はプロンプトキャッシュに当たりやすく、レイテンシが実態より
// 短く出る。周回にすると同一ケースの再実行が時間的に離れる。
//
// progress へ1件ごとの経過を書く（nil なら書かない）。
//
// 設定の誤り（認証エラー・APIキー未設定）を検知したら即座に打ち切る。
// 続行すると「破損率100%」というもっともらしい数字が出て、
// 設定ミスを品質の問題と読み違える。
func Run(ctx context.Context, t Target, cases []Case, runs int, progress io.Writer) ([]Observation, error) {
	if runs < 1 {
		runs = 1
	}
	obs := make([]Observation, 0, len(cases)*runs)
	for run := range runs {
		for _, c := range cases {
			o := t.Run(ctx, c)
			o.Run = run + 1
			if o.Fatal {
				return obs, fmt.Errorf("%s の呼び出しが設定の問題で失敗しました（品質の測定になっていません）: %s", c.ID, o.Error)
			}
			obs = append(obs, o)
			if progress != nil {
				mark := "ok"
				if o.Broken {
					mark = "破損:" + o.BrokenReason
				} else if len(o.Violations) > 0 {
					mark = fmt.Sprintf("違反%d", len(o.Violations))
				}
				fmt.Fprintf(progress, "  %-12s run%d %-10s score=%.2f %5dms %s\n",
					c.ID, run+1, c.Label, o.Score, o.LatencyMS, mark)
			}
		}
	}
	return obs, nil
}

// RunAndAggregate は実行から集計までをまとめる。
func RunAndAggregate(ctx context.Context, t Target, cases []Case, runs int, manifestPath string, progress io.Writer) (*Summary, error) {
	obs, err := Run(ctx, t, cases, runs, progress)
	if err != nil {
		return nil, err
	}
	s := Aggregate(t.Name(), t.Model(), t.Endpoint(), time.Now().UTC().Format(time.RFC3339), obs)
	s.ManifestPath = manifestPath
	s.ManifestSHA256 = fileSHA256(manifestPath)
	return s, nil
}

// EstimateCost は実行前の概算コストを返す（件数 × 回数）。
//
// 実行前に出すのは、間違ったマニフェストで数千件を回す事故を止めるため。
// 出力トークンは上限いっぱいを仮定するので、実際はこれより安くなる。
func EstimateCost(t Target, cases []Case, runs int) float64 {
	if runs < 1 {
		runs = 1
	}
	var prompt, completion int
	for _, c := range cases {
		p, cm := t.EstimateTokens(c)
		prompt += p * runs
		completion += cm * runs
	}
	return costs.EstimateCostUSD(t.Model(), prompt, completion)
}

// fileSHA256 はマニフェストの同一性を示すハッシュ。読めなければ空文字。
func fileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}
