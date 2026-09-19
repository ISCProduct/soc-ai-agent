package resume

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestBothReviewPathsCallPersistReview は「保存を呼び忘れる」形への回帰を止める。
//
// #1332 は persistReview の中身の不具合ではなく、ストリーム経路が保存を
// 一度も呼んでいなかったという欠陥だった。persistReview の単体テストだけでは
// 呼び出しごと消す変更を検出できないので、呼び出しの存在そのものを固定する。
//
// AI クライアントのスタブ化なしに両経路を通すのは高くつくため、構文木で見る。
func TestBothReviewPathsCallPersistReview(t *testing.T) {
	const src = "resume_review.go"

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, src, nil, 0)
	if err != nil {
		t.Fatalf("%s のパースに失敗: %v", src, err)
	}

	calls := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "persistReview" {
				calls[fn.Name.Name] = true
			}
			return true
		})
	}

	for _, fn := range []string{"ReviewDocument", "ReviewDocumentStream"} {
		t.Run(fn, func(t *testing.T) {
			if !calls[fn] {
				t.Fatalf("%s が persistReview を呼んでいない。"+
					"レビュー結果が保存されないまま画面にだけ出る（#1332 の再発）", fn)
			}
		})
	}
}
