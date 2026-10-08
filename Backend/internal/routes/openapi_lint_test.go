package routes

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestOpenAPILint openapi.yaml の体裁を固定する。
//
// 網羅率が 100% になっても、中身が薄いと読み物として機能しない。
// 実際に初期に書いた 17 本は security を省略していて、「意図的に公開」なのか
// 「書き忘れ」なのか区別できない状態だった（OpenAPI では省略はグローバル継承を
// 意味するため、後からグローバル security を足すと挙動が変わる）。
// 以下を全 operation の最低条件として固定する。
//
//   - summary がある
//   - tags がある。宣言済みのタグだけを使う
//   - security がある。公開なら明示的に空配列を書く
//   - responses がある
//   - $ref がすべて解決する
//   - 宣言したタグが全て使われている
func TestOpenAPILint(t *testing.T) {
	raw, err := os.ReadFile(openapiPath)
	if err != nil {
		t.Fatalf("%s が読めない: %v", openapiPath, err)
	}
	var d map[string]any
	if err := yaml.Unmarshal(raw, &d); err != nil {
		t.Fatalf("openapi.yaml を解釈できない: %v", err)
	}

	var refs []string
	var walk func(any)
	walk = func(n any) {
		switch v := n.(type) {
		case map[string]any:
			for k, vv := range v {
				if k == "$ref" {
					if s, ok := vv.(string); ok {
						refs = append(refs, s)
					}
					continue
				}
				walk(vv)
			}
		case []any:
			for _, vv := range v {
				walk(vv)
			}
		}
	}
	walk(d)

	resolve := func(ref string) bool {
		if !strings.HasPrefix(ref, "#/") {
			return false
		}
		var cur any = d
		for _, seg := range strings.Split(ref[2:], "/") {
			m, ok := cur.(map[string]any)
			if !ok {
				return false
			}
			if cur, ok = m[seg]; !ok {
				return false
			}
		}
		return true
	}
	for _, r := range refs {
		if !resolve(r) {
			t.Errorf("解決できない $ref: %s", r)
		}
	}

	declaredTags := map[string]bool{}
	for _, tg := range d["tags"].([]any) {
		declaredTags[tg.(map[string]any)["name"].(string)] = true
	}
	schemes := map[string]bool{}
	for k := range d["components"].(map[string]any)["securitySchemes"].(map[string]any) {
		schemes[k] = true
	}

	usedTags := map[string]bool{}
	ops := 0
	for path, pv := range d["paths"].(map[string]any) {
		for method, ov := range pv.(map[string]any) {
			switch method {
			case "get", "post", "put", "patch", "delete":
			default:
				continue
			}
			ops++
			at := strings.ToUpper(method) + " " + path
			o, ok := ov.(map[string]any)
			if !ok {
				t.Errorf("operation の形が不正: %s", at)
				continue
			}
			if o["summary"] == nil {
				t.Errorf("summary が無い: %s", at)
			}
			if o["responses"] == nil {
				t.Errorf("responses が無い: %s", at)
			}
			tags, _ := o["tags"].([]any)
			if len(tags) == 0 {
				t.Errorf("tags が無い: %s", at)
			}
			for _, tv := range tags {
				name, _ := tv.(string)
				usedTags[name] = true
				if !declaredTags[name] {
					t.Errorf("tags に未宣言のタグ %q: %s", name, at)
				}
			}
			sec, has := o["security"]
			if !has {
				t.Errorf("security が無い: %s（公開なら `security: []` と明示する）", at)
			}
			for _, sv := range toList(sec) {
				m, _ := sv.(map[string]any)
				for name := range m {
					if !schemes[name] {
						t.Errorf("未宣言の securityScheme %q: %s", name, at)
					}
				}
			}
		}
	}
	for tg := range declaredTags {
		if !usedTags[tg] {
			t.Errorf("宣言されているが使われていないタグ: %s", tg)
		}
	}
	t.Logf("operations %d / tags %d / securitySchemes %d / $ref %d", ops, len(declaredTags), len(schemes), len(refs))
}

func toList(v any) []any {
	l, _ := v.([]any)
	return l
}
