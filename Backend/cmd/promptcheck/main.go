// Command promptcheck はプロンプト版の環境変数だけを確かめる（SOCAIAGENT-661）。
//
// Backend は起動時に同じ検証をして、不正なら起動を止める。本番のデプロイでは、サービスを
// 差し替える前に新しいイメージとタスク定義の環境変数でこれを実行し、起動できない設定を先に見つける。
//
//	docker run --rm --entrypoint /bin/promptcheck -e PROMPT_VERSION=v2 <image>
//	go run ./cmd/promptcheck   # ローカル（環境変数をそのまま読む。.env は読まない）
package main

import (
	"fmt"
	"os"

	"Backend/internal/services/prompts"
)

func main() {
	if err := prompts.ValidateSettings(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("プロンプト版の設定は有効です")
}
