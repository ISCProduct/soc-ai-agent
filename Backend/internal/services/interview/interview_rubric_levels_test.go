package interview

import (
	"os"
	"strings"
	"testing"
)

// TestRubricLevelsDefined はレベル定義の書き忘れと重複を止める（#1594）。
//
// Levels は配列なので、短いコンポジットリテラルを書くとコンパイラは
// 何も言わずに残りをゼロ値（空文字）で埋める。空の段があると
// 「満たしている最も高いレベル」をモデルが選べない。
func TestRubricLevelsDefined(t *testing.T) {
	for _, c := range RubricCriteria() {
		t.Run(c.Key, func(t *testing.T) {
			seen := map[string]int{}
			for score, level := range c.Levels {
				if strings.TrimSpace(level) == "" {
					t.Errorf("%d点のレベル定義が空", score)
					continue
				}
				if prev, dup := seen[level]; dup {
					t.Errorf("%d点と%d点のレベル定義が同一（隣接レベルを判別できない）: %q", prev, score, level)
				}
				seen[level] = score
			}
		})
	}
}

// TestRubricPromptSectionGolden はプロンプトの評価基準セクションを1文字も違わず固定する。
//
// **これが無いとレベル定義を逆順（0点＝最良）にしても全テストが通る。**
// 他のテストは期待値を c.Levels 自身から取って部分一致を見るだけなので、
// 添字と本文の対応を誰も見ていない。採点が反転したルーブリックは
// user_weight_scores まで流れ、マッチングと教員向け傾向分析の両方に
// 静かに混ざる（docs/wiki/scoring.md §2-3）。
//
// レベル定義を意図して変えたときは、失敗出力に出る実際のセクションを
// testdata/rubric_prompt.golden へ丸ごと置き換え、**弁別力をハーネスで
// 測り直してから**コミットすること。
func TestRubricPromptSectionGolden(t *testing.T) {
	const goldenPath = "testdata/rubric_prompt.golden"
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden の読み込みに失敗: %v", err)
	}
	got := BuildRubricPromptSection()
	if got != string(want) {
		t.Errorf("プロンプトの評価基準セクションが golden と一致しない。\n"+
			"意図した変更なら %s を下記の内容へ置き換え、ハーネスで弁別力を測り直すこと。\n"+
			"--- 実際の出力 ---\n%s\n--- golden ---\n%s", goldenPath, got, want)
	}
}

// TestBuildRubricPromptSectionIncludesLevels はレベル定義がプロンプトへ実際に入ることを見る。
// 定義を足しても BuildRubricPromptSection が使わなければ意味が無い。
func TestBuildRubricPromptSectionIncludesLevels(t *testing.T) {
	section := BuildRubricPromptSection()
	for _, c := range RubricCriteria() {
		for _, want := range []string{c.Key, c.Label, c.Description} {
			if !strings.Contains(section, want) {
				t.Errorf("プロンプトに %q が含まれない", want)
			}
		}
		for score, level := range c.Levels {
			if !strings.Contains(section, level) {
				t.Errorf("%s の %d点のレベル定義がプロンプトに含まれない: %q", c.Key, score, level)
			}
		}
	}
}
