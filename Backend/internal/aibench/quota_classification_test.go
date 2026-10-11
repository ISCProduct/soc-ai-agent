package aibench

import (
	"bytes"
	"net/http"
	"testing"
)

// 429 は一時的なレート制限と残高切れの両方に使われる（#1634）。
// 残高切れをネットワーク系に寄せていたため、残高が尽きた実行が全件失敗のまま
// 完走し「破損率 0.0% / 弁別力 +0.000」という体裁の整った報告を出していた。
// どちらも品質が良いときの表示と見分けが付かない。
func TestQuotaClassification(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		body           string
		wantSetup      bool // 即座に打ち切る（設定の問題）
		wantUnmeasured bool // 測り直せば消える
	}{
		{
			name:           "429 残高切れ(code)は打ち切る",
			status:         http.StatusTooManyRequests,
			body:           `{"error":{"message":"You exceeded your current quota","type":"insufficient_quota","code":"insufficient_quota"}}`,
			wantSetup:      true,
			wantUnmeasured: false,
		},
		{
			name:           "429 残高切れ(credit_balance_exhausted)は打ち切る",
			status:         http.StatusTooManyRequests,
			body:           `{"error":{"code":"credit_balance_exhausted"}}`,
			wantSetup:      true,
			wantUnmeasured: false,
		},
		{
			name:           "429 レート制限は再試行する",
			status:         http.StatusTooManyRequests,
			body:           `{"error":{"message":"Rate limit reached","type":"requests","code":"rate_limit_exceeded"}}`,
			wantSetup:      false,
			wantUnmeasured: true,
		},
		{
			name:           "429 でJSONが壊れていても本文に残高切れがあれば打ち切る",
			status:         http.StatusTooManyRequests,
			body:           `insufficient_quota`,
			wantSetup:      true,
			wantUnmeasured: false,
		},
		{
			name:           "429 で判別材料が無ければ再試行側に倒す",
			status:         http.StatusTooManyRequests,
			body:           `{"error":{"message":"slow down"}}`,
			wantSetup:      false,
			wantUnmeasured: true,
		},
		{
			name:           "401 は従来どおり打ち切る",
			status:         http.StatusUnauthorized,
			body:           `{"error":{"code":"invalid_api_key"}}`,
			wantSetup:      true,
			wantUnmeasured: false,
		},
		{
			name:           "500 は従来どおり再試行する",
			status:         http.StatusInternalServerError,
			body:           `{}`,
			wantSetup:      false,
			wantUnmeasured: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(tt.body)
			if got := isSetupFailure(tt.status, body); got != tt.wantSetup {
				t.Errorf("isSetupFailure = %v, want %v", got, tt.wantSetup)
			}
			if got := isUnmeasurable(tt.status, body); got != tt.wantUnmeasured {
				t.Errorf("isUnmeasurable = %v, want %v", got, tt.wantUnmeasured)
			}
			// 打ち切りと再試行は排他。両方 true だと分類の意味が無くなる。
			if tt.wantSetup && tt.wantUnmeasured {
				t.Fatal("テストの期待値が矛盾している")
			}
		})
	}
}

// 1件も計測できていない実行で指標を出さない（#1634）。
func TestPrintSummarySuppressesMetricsWhenNothingMeasured(t *testing.T) {
	var buf bytes.Buffer
	PrintSummary(&buf, &Summary{
		Target:         "resume",
		Model:          "gpt-4o-mini",
		MeasuredRuns:   0,
		UnmeasuredRuns: 92,
		BrokenByReason: map[string]int{string(BrokenNetwork): 92},
	})
	out := buf.String()

	for _, ng := range []string{"破損率", "弁別力", "再現性"} {
		if bytes.Contains([]byte(out), []byte(ng)) {
			t.Errorf("計測0件の実行で %q を表示してはいけない:\n%s", ng, out)
		}
	}
	for _, want := range []string{"計測できた呼び出しが1件もありません", "92回"} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("%q を表示すること:\n%s", want, out)
		}
	}
}

// 計測できていれば従来どおり指標を出す。
func TestPrintSummaryShowsMetricsWhenMeasured(t *testing.T) {
	var buf bytes.Buffer
	PrintSummary(&buf, &Summary{
		Target: "resume", Model: "gpt-4o-mini", MeasuredRuns: 10, Cases: 5, Runs: 2,
	})
	if !bytes.Contains(buf.Bytes(), []byte("破損率")) {
		t.Errorf("計測できた実行では指標を出すこと:\n%s", buf.String())
	}
}
