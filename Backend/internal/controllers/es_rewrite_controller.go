package controllers

import (
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type ESRewriteController struct {
	openaiClient *openai.Client
}

func NewESRewriteController(openaiClient *openai.Client) *ESRewriteController {
	return &ESRewriteController{openaiClient: openaiClient}
}

type esRewriteRequest struct {
	OriginalText string `json:"original_text"`
	QuestionType string `json:"question_type"` // "志望動機" | "自己PR" | "学チカ" | "その他"
	TechStack    string `json:"tech_stack"`    // 任意: 使用技術スタック
	CompanyName  string `json:"company_name"`  // 任意: 志望企業名（Web Search を利用する場合）
}

type starBreakdown struct {
	Situation string `json:"situation"`
	Task      string `json:"task"`
	Action    string `json:"action"`
	Result    string `json:"result"`
}

type esRewriteResponse struct {
	RewrittenText string        `json:"rewritten_text"`
	Star          starBreakdown `json:"star"`
}

func (c *ESRewriteController) Rewrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req esRewriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.OriginalText = strings.TrimSpace(req.OriginalText)
	if req.OriginalText == "" {
		http.Error(w, "original_text is required", http.StatusBadRequest)
		return
	}
	if req.QuestionType == "" {
		req.QuestionType = "その他"
	}

	systemPrompt := `あなたはエンジニア就職活動の専門アドバイザーです。
学生が書いたES文章を、採用担当者に刺さるエンジニア向けの表現にリライトしてください。
JSONのみで返してください。`

	techInfo := ""
	if req.TechStack != "" {
		techInfo = "\n使用技術スタック（参考）: " + req.TechStack
	}

	// 企業名が指定されていれば Web Search を使って最新の企業情報を取得し、プロンプトに注入する（失敗しても処理を継続）
	companyInfo := ""
	if strings.TrimSpace(req.CompanyName) != "" {
		if info, err := c.openaiClient.WebSearchQuery(context.Background(), "企業名: "+req.CompanyName+" 採用情報 求める人物像 企業理念"); err == nil {
			if len(info) > 2000 {
				info = info[:2000]
			}
			companyInfo = "\n\n【企業情報（WebSearch）】\n" + info
		}
	}

	userPrompt := `以下のES文章を、STAR法（Situation/Task/Action/Result）に沿ったエンジニア採用向けの表現にリライトしてください。

【質問種別】` + req.QuestionType + `
【元のES文章】
` + req.OriginalText + techInfo + companyInfo + `

## リライトのルール
- 「頑張りました」「工夫しました」等の抽象表現を、具体的な技術・数値・成果に置き換える
- STAR法: Situation（状況）/ Task（課題）/ Action（技術的施策）/ Result（成果・数値）の構造で記述する
- エンジニア採用に刺さる技術的な動詞・名詞を使用する（実装した、設計した、最適化した、削減した等）
- 元の内容を大きく変えず、言語化を強化する方向でリライトする
- 文字数は元の文章の120〜150%程度を目安にする

## 出力フォーマット（このキーと型を厳守）
{
  "rewritten_text": "リライト後の完成文章",
  "star": {
    "situation": "状況（背景・前提）の部分の説明",
    "task": "課題・目標の部分の説明",
    "action": "技術的な施策・行動の部分の説明",
    "result": "成果・結果の部分の説明"
  }
}`

	raw, err := c.openaiClient.ChatCompletionJSON(context.Background(), systemPrompt, userPrompt, 0.7, 1500)
	if err != nil {
		writeInternalServerError(w, err)
		return
	}

	// Extract JSON object in case of markdown fences
	cleaned := extractESJSON(raw)

	var resp esRewriteResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		http.Error(w, "Failed to parse AI response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// extractESJSON strips markdown code fences and extracts the outermost JSON object.
func extractESJSON(raw string) string {
	s := strings.TrimSpace(raw)
	if start := strings.Index(s, "{"); start > 0 {
		s = s[start:]
	}
	if end := strings.LastIndex(s, "}"); end >= 0 && end < len(s)-1 {
		s = s[:end+1]
	}
	return s
}
