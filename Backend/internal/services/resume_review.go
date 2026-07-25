package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"Backend/internal/models"
	"Backend/internal/ragclient"
)

func (s *ResumeService) ReviewDocument(documentID uint, requestingUserID uint, companyName string, jobTitle string, candidateType string) (*models.ResumeReview, []models.ResumeReviewItem, error) {
	doc, err := s.repo.FindDocumentByID(documentID)
	if err != nil {
		return nil, nil, err
	}
	if doc.UserID != requestingUserID {
		return nil, nil, ErrForbidden
	}
	if strings.TrimSpace(companyName) == "" && strings.TrimSpace(jobTitle) == "" {
		return nil, nil, &ValidationError{Message: "応募企業名または応募職種を入力してください"}
	}
	canonicalName, err := s.ensureRealCompany(context.Background(), companyName)
	if err != nil {
		return nil, nil, err
	}
	companyName = canonicalName

	workDir, err := s.ensureWorkingDir(doc.ID)
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(workDir)

	pdfPath, normalizedStored, err := s.normalizeToPDF(doc, workDir)
	if err != nil {
		return nil, nil, err
	}
	doc.NormalizedPath = normalizedStored
	doc.Status = "normalized"
	if err := s.repo.UpdateDocument(doc); err != nil {
		return nil, nil, err
	}

	if err := validatePathInDir(pdfPath, workDir); err != nil {
		return nil, nil, fmt.Errorf("invalid pdf path: %w", err)
	}
	blocks, err := s.extractTextBlocks(doc, pdfPath)
	if err != nil {
		return nil, nil, err
	}
	hasText := false
	for _, block := range blocks {
		if strings.TrimSpace(block.Text) != "" {
			hasText = true
			break
		}
	}
	if !hasText {
		return nil, nil, &ValidationError{Message: "履歴書からテキストを抽出できませんでした。PDF の画質や形式を確認してください"}
	}
	if err := s.repo.ReplaceTextBlocks(doc.ID, blocks); err != nil {
		return nil, nil, err
	}

	review, items, err := s.buildResumeReviewWithAI(blocks, companyName, jobTitle, candidateType)
	if err != nil {
		return nil, nil, err
	}
	review.DocumentID = doc.ID
	if err := s.repo.CreateReview(review); err != nil {
		return nil, nil, err
	}
	for i := range items {
		items[i].ReviewID = review.ID
	}
	if err := s.repo.ReplaceReviewItems(review.ID, items); err != nil {
		return nil, nil, err
	}

	// スコア更新（クロス機能連携）
	if s.crossFeature != nil {
		if err := s.crossFeature.UpdateScoresFromResumeReview(doc.UserID, doc.SessionID, review, items); err != nil {
			log.Printf("[Resume] crossFeature score update failed: %v\n", err)
		}
	}

	annotatedPath, annotatedStored, err := s.annotatePDF(pdfPath, doc, review, items)
	if err != nil {
		return review, items, err
	}
	_ = annotatedPath
	doc.AnnotatedPath = annotatedStored
	doc.Status = "reviewed"
	if err := s.repo.UpdateDocument(doc); err != nil {
		return review, items, err
	}

	return review, items, nil
}

type aiReviewResponse struct {
	Score          int            `json:"score"`
	Summary        string         `json:"summary"`
	CompanySummary string         `json:"company_summary,omitempty"`
	Items          []aiReviewItem `json:"items"`
}

type aiReviewItem struct {
	Quote      string `json:"quote"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
	Severity   string `json:"severity"`
	PageHint   int    `json:"page_hint,omitempty"`
	BlockIndex int    `json:"block_index,omitempty"`
}

type ragReviewRequest struct {
	ResumeText     string `json:"resume_text"`
	CompanyName    string `json:"company_name"`
	JobTitle       string `json:"job_title"`
	CompanyContext string `json:"company_context,omitempty"`
}

type ragReviewResponse struct {
	Report string `json:"report"`
}

func (s *ResumeService) fetchRAGReport(resumeText, companyName, jobTitle string) (string, error) {
	baseURL := strings.TrimSpace(os.Getenv("RAG_REVIEW_URL"))
	if baseURL == "" {
		return "", errors.New("RAG_REVIEW_URL is not set")
	}

	log.Printf("resume_review: rag request company=%q job_title=%q", companyName, jobTitle)

	payload := ragReviewRequest{
		ResumeText:     resumeText,
		CompanyName:    companyName,
		JobTitle:       jobTitle,
		CompanyContext: s.lookupCompanyBriefFromCache(companyName),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := strings.TrimRight(baseURL, "/") + "/resume/review"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	ragclient.SetAuthHeader(req)

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("rag review failed: %s", strings.TrimSpace(string(respBody)))
	}

	response := ragReviewResponse{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.Report) == "" {
		return "", errors.New("rag report is empty")
	}
	log.Printf("resume_review: rag response length=%d", len(response.Report))
	return response.Report, nil
}

// fetchRAGReportStream は FastAPI ストリーミングエンドポイントを呼び出し、
// SSE レスポンスボディ (io.ReadCloser) を返す。呼び出し元がクローズする責務を持つ。
func (s *ResumeService) fetchRAGReportStream(ctx context.Context, resumeText, companyName, jobTitle string) (io.ReadCloser, error) {
	baseURL := strings.TrimSpace(os.Getenv("RAG_REVIEW_URL"))
	if baseURL == "" {
		return nil, errors.New("RAG_REVIEW_URL is not set")
	}

	payload := ragReviewRequest{
		ResumeText:     resumeText,
		CompanyName:    companyName,
		JobTitle:       jobTitle,
		CompanyContext: s.lookupCompanyBriefFromCache(companyName),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(baseURL, "/") + "/resume/review/stream"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	ragclient.SetAuthHeader(req)

	// ストリーミングのためタイムアウトなし
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("rag stream request failed: status %d", resp.StatusCode)
	}
	return resp.Body, nil
}

// ReviewDocumentStream はドキュメントを前処理した後、SSEでRAGレポートをストリーミングし、
// 最後にスコア・指摘事項を complete イベントとして送信する。
func (s *ResumeService) ReviewDocumentStream(ctx context.Context, documentID uint, requestingUserID uint, companyName, jobTitle, candidateType string, w http.ResponseWriter) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	sendEvent := func(v map[string]any) {
		data, _ := json.Marshal(v)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	doc, err := s.repo.FindDocumentByID(documentID)
	if err != nil {
		sendEvent(map[string]any{"type": "error", "message": err.Error()})
		return err
	}
	if doc.UserID != requestingUserID {
		sendEvent(map[string]any{"type": "error", "message": "forbidden"})
		return ErrForbidden
	}
	if strings.TrimSpace(companyName) == "" && strings.TrimSpace(jobTitle) == "" {
		msg := "応募企業名または応募職種を入力してください"
		sendEvent(map[string]any{"type": "error", "message": msg})
		return errors.New(msg)
	}
	canonicalName, err := s.ensureRealCompany(ctx, companyName)
	if err != nil {
		msg := err.Error()
		var ve *ValidationError
		if errors.As(err, &ve) {
			msg = ve.Message
		}
		sendEvent(map[string]any{"type": "error", "message": msg})
		return err
	}
	companyName = canonicalName

	workDir, err := s.ensureWorkingDir(doc.ID)
	if err != nil {
		sendEvent(map[string]any{"type": "error", "message": err.Error()})
		return err
	}
	defer os.RemoveAll(workDir)

	pdfPath, normalizedStored, err := s.normalizeToPDF(doc, workDir)
	if err != nil {
		sendEvent(map[string]any{"type": "error", "message": err.Error()})
		return err
	}
	doc.NormalizedPath = normalizedStored
	doc.Status = "normalized"
	_ = s.repo.UpdateDocument(doc)

	if err := validatePathInDir(pdfPath, workDir); err != nil {
		sendEvent(map[string]any{"type": "error", "message": "document processing failed"})
		return fmt.Errorf("invalid pdf path: %w", err)
	}
	blocks, err := s.extractTextBlocks(doc, pdfPath)
	if err != nil {
		sendEvent(map[string]any{"type": "error", "message": err.Error()})
		return err
	}
	hasText := false
	for _, b := range blocks {
		if strings.TrimSpace(b.Text) != "" {
			hasText = true
			break
		}
	}
	if !hasText {
		msg := "履歴書からテキストを抽出できませんでした。PDF の画質や形式を確認してください"
		sendEvent(map[string]any{"type": "error", "message": msg})
		return errors.New(msg)
	}
	_ = s.repo.ReplaceTextBlocks(doc.ID, blocks)

	text := buildResumeText(blocks, 30000)

	// RAGレポートをストリーミングしつつ全文を収集する
	var ragReport string
	if strings.TrimSpace(companyName) != "" {
		ragBody, ragErr := s.fetchRAGReportStream(ctx, text, companyName, jobTitle)
		if ragErr == nil {
			ragReport, _ = relaySSEChunks(ragBody, w, flusher)
			ragBody.Close()
		} else {
			log.Printf("resume_review_stream: rag stream failed: %v", ragErr)
		}
	}

	// スコア・指摘事項を生成
	review, items, err := s.buildReviewScoreItems(blocks, companyName, jobTitle, candidateType, ragReport)
	if err != nil {
		log.Printf("resume_review_stream: build score failed: %v", err)
		var ve *ValidationError
		if errors.As(err, &ve) {
			sendEvent(map[string]any{"type": "error", "message": ve.Message})
		} else {
			sendEvent(map[string]any{"type": "error", "message": err.Error()})
		}
		return err
	}

	// 注釈 PDF を生成して保存
	annotatedAvailable := false
	_, annotatedStored, err := s.annotatePDF(pdfPath, doc, review, items)
	if err != nil {
		log.Printf("resume_review_stream: annotatePDF failed document_id=%d err=%v", documentID, err)
		sendEvent(map[string]any{
			"type":    "annotate_error",
			"message": "注釈PDFの生成に失敗しました。レビュー結果は表示されますが、PDFダウンロードはご利用いただけません。",
		})
	} else {
		doc.AnnotatedPath = annotatedStored
		doc.Status = "reviewed"
		_ = s.repo.UpdateDocument(doc)
		annotatedAvailable = true
	}

	sendEvent(map[string]any{
		"type":                "complete",
		"review":              review,
		"items":               items,
		"annotated_available": annotatedAvailable,
	})
	return nil
}

// relaySSEChunks はFastAPIからのSSEストリームを読み取り、chunk イベントをそのまま
// クライアントに転送しつつ、全テキストを返す。
func relaySSEChunks(body io.Reader, w io.Writer, flusher http.Flusher) (string, error) {
	var accum strings.Builder
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := line[6:]

		var event struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}

		switch event.Type {
		case "chunk":
			accum.WriteString(event.Text)
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		case "done":
			return accum.String(), nil
		case "error":
			return accum.String(), fmt.Errorf("rag stream error: %s", event.Message)
		}
	}
	return accum.String(), scanner.Err()
}

func (s *ResumeService) buildResumeReviewWithAI(blocks []models.ResumeTextBlock, companyName string, jobTitle string, candidateType string) (*models.ResumeReview, []models.ResumeReviewItem, error) {
	text := buildResumeText(blocks, 30000)
	if strings.TrimSpace(text) == "" {
		return nil, nil, &ValidationError{Message: "履歴書からテキストを抽出できませんでした。PDF の画質や形式を確認してください"}
	}
	if s.aiClient == nil {
		return nil, nil, fmt.Errorf("AIクライアントが初期化されていません")
	}

	var companyInfo string
	if strings.TrimSpace(companyName) != "" {
		if ragReport, err := s.fetchRAGReport(text, companyName, jobTitle); err == nil {
			companyInfo = ragReport
		} else {
			log.Printf("resume_review: rag report failed: %v", err)
		}
	}
	return s.buildReviewScoreItems(blocks, companyName, jobTitle, candidateType, companyInfo)
}

// buildReviewScoreItems はcompanyInfoを受け取りOpenAIでスコア・指摘事項を生成する。
// fetchRAGReportStream など外部から取得したRAGレポートを直接渡す場合に使用する。
func (s *ResumeService) buildReviewScoreItems(blocks []models.ResumeTextBlock, companyName, jobTitle, candidateType, companyInfo string) (*models.ResumeReview, []models.ResumeReviewItem, error) {
	if s.aiClient == nil {
		return nil, nil, fmt.Errorf("AIクライアントが初期化されていません")
	}
	text := buildResumeText(blocks, 30000)
	if strings.TrimSpace(companyName) != "" && strings.TrimSpace(companyInfo) == "" {
		// 共有キャッシュのみ。未登録時は空（Search / 都度 LLM 企業調査はしない）
		if brief := s.lookupCompanyBriefFromCache(companyName); brief != "" {
			companyInfo = brief
		}
	}

	prompt := fmt.Sprintf(`以下は履歴書/エントリーシートのOCRテキストです。
この内容をレビューし、改善すべき点を最大8件までJSONで返してください。
必ず本文中に存在する短い引用(quote)を入れてください。quoteは後で位置合わせに使います。
「記載されていません」「未記入」などの欠落指摘は禁止です。本文の内容に基づいた具体的な改善点のみを書いてください。
page_hintは本文の行頭にある [P#B#] の P# を使ってください。
block_indexは本文の行頭にある [P#B#] の B# を使ってください。
各itemsは必ず本文の1ブロックに対応させ、総合的なまとめや全体評価だけの項目は禁止です。
messageとsuggestionは該当ブロックの内容を引用・要約して具体的に指摘してください。
suggestionは「どう直すか」が分かるように書いてください（数値・役割・成果・再現性など具体語を含める）。

応募企業名: %s
応募職種: %s
企業情報(参考): %s
候補者区分: %s
企業名が空欄の場合は一般的な観点でレビューしてください。
学歴/職歴は明らかな矛盾・不足がある場合のみ指摘し、それ以外は指摘から除外してください。
企業に合わせた観点（求める人物像・事業領域・評価軸）に照らし、応募書類の内容がどう評価されるかを具体的に指摘してください。
一般論ではなく、この応募企業に合わせた改善提案を優先してください。

出力は次のJSONのみ:
{"score":0-100,"summary":"短い要約","items":[{"quote":"本文中の一文","message":"指摘","suggestion":"改善案","severity":"info|warning|critical","page_hint":1,"block_index":1}]}

OCRテキスト:
%s`, companyName, jobTitle, companyInfo, candidateType, text)

	modelOverride := strings.TrimSpace(os.Getenv("OPENAI_REVIEW_MODEL"))
	if modelOverride == "" {
		modelOverride = "gpt-4o-mini"
	}
	raw, err := s.aiClient.ResponsesWithMaxTokens(context.Background(), "あなたは日本語の履歴書・エントリーシートを添削する専門家です。必ず具体的な書き換え案をJSON形式で提示します。", prompt, 0.2, 2000, modelOverride)
	if err != nil {
		log.Printf("resume_review: openai review failed: %v", err)
		return nil, nil, fmt.Errorf("AIレビューの生成に失敗しました。しばらく待ってから再度お試しください")
	}

	response := aiReviewResponse{}
	if err := decodeJSON(raw, &response); err != nil {
		log.Printf("resume_review: decode failed: %v", err)
		return nil, nil, fmt.Errorf("AIレビュー結果の解析に失敗しました。再度お試しください")
	}

	if response.Score <= 0 {
		response.Score = 70
	}
	if response.Summary == "" {
		response.Summary = "内容を確認しました。具体性と成果の明確化が改善ポイントです。"
	}

	items := mapReviewItems(blocks, response.Items)
	log.Printf("resume_review: items mapped=%d raw=%d", len(items), len(response.Items))
	if len(items) < 3 {
		blocksForRetry := selectReviewBlocks(blocks, 40)
		blockList := buildBlockList(blocksForRetry)
		retryPrompt := fmt.Sprintf(`以下のブロック一覧から、各ブロックに必ず紐づく指摘を最大8件返してください。
各itemsは必ず block_index と page_hint を含め、quote は block_text の一部をそのまま抜粋してください。
総合的なまとめや全体評価は不可です。必ずブロック単位で具体的に指摘してください。
 suggestionは具体的な書き換え案にしてください（数値・役割・成果・再現性を含める）。

応募企業名: %s
応募職種: %s
企業情報(参考): %s
候補者区分: %s
学歴/職歴は明らかな矛盾・不足がある場合のみ指摘し、それ以外は指摘から除外してください。

ブロック一覧:
%s

出力は次のJSONのみ:
{"score":0-100,"summary":"短い要約","items":[{"quote":"本文中の一文","message":"指摘","suggestion":"改善案","severity":"info|warning|critical","page_hint":1,"block_index":1}]}`,
			companyName, jobTitle, companyInfo, candidateType, blockList)
		rawRetry, err := s.aiClient.ResponsesWithMaxTokens(context.Background(), "あなたは日本語の履歴書・エントリーシートを添削する専門家です。JSON形式で出力してください。", retryPrompt, 0.2, 2000, modelOverride)
		if err == nil {
			responseRetry := aiReviewResponse{}
			if decodeJSON(rawRetry, &responseRetry) == nil {
				items = mapReviewItems(blocks, responseRetry.Items)
				log.Printf("resume_review: retry items mapped=%d raw=%d", len(items), len(responseRetry.Items))
			}
		}
	}
	if len(items) == 0 {
		log.Println("resume_review: no items could be mapped after retry")
		return nil, nil, fmt.Errorf("AIが生成した指摘内容を履歴書ブロックに紐づけられませんでした。再度お試しください")
	}

	return &models.ResumeReview{
		Score:   response.Score,
		Summary: response.Summary,
	}, items, nil
}
