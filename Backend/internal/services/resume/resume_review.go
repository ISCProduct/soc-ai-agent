package resume

import (
	"Backend/internal/services/shared"
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
	"Backend/internal/openai"
	"Backend/internal/ragclient"

	"gorm.io/gorm"
)

// ctx はリクエストIDを RAG まで伝播させるために受け取る(#1188)
func (s *ResumeService) ReviewDocument(ctx context.Context, documentID uint, requestingUserID uint, companyName string, jobTitle string, candidateType string) (*models.ResumeReview, []models.ResumeReviewItem, error) {
	// クエリを user_id でスコープする。所有者以外には見つからない（#1156）
	doc, err := s.repo.FindDocumentByIDForUser(documentID, requestingUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, shared.ErrForbidden
		}
		return nil, nil, err
	}
	if strings.TrimSpace(companyName) == "" && strings.TrimSpace(jobTitle) == "" {
		return nil, nil, &shared.ValidationError{Message: "応募企業名または応募職種を入力してください"}
	}
	canonicalName, err := s.ensureRealCompany(ctx, companyName)
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
		return nil, nil, &shared.ValidationError{Message: "履歴書からテキストを抽出できませんでした。PDF の画質や形式を確認してください"}
	}
	if err := s.repo.ReplaceTextBlocks(doc.ID, blocks); err != nil {
		return nil, nil, err
	}

	review, items, err := s.buildResumeReviewWithAI(ctx, blocks, companyName, jobTitle, candidateType)
	if err != nil {
		return nil, nil, err
	}
	if err := s.persistReview(doc, review, items); err != nil {
		return nil, nil, err
	}

	if _, err := s.finalizeDocument(doc, pdfPath, review, items); err != nil {
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

// ctx はリクエストIDを RAG へ伝播させるために受け取る(#1188)
func (s *ResumeService) fetchRAGReport(ctx context.Context, resumeText, companyName, jobTitle string) (string, error) {
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
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
// finalizeDocument は注釈PDFを作り、status を reviewed にして保存する。
// 戻り値は「注釈PDFが使えるか」と、status 保存の失敗。
//
// 注釈は付加機能なので、失敗してもレビュー済みであることは変わらない。
// ここで早期 return して status を保存しそこねると、レビューは保存済みなのに
// 教員一覧と統合プロファイル(doc.Status == "reviewed" を見ている)に
// 出てこない。通常経路とストリーム経路で2度これを踏んだので処理を1つにする。
//
// 注釈に失敗したら AnnotatedPath は空にする。再レビュー時に前回の注釈PDFが
// 残っていると、新しいレビュー本文と古い指摘入りPDFが組で配られる。
func (s *ResumeService) finalizeDocument(
	doc *models.ResumeDocument,
	pdfPath string,
	review *models.ResumeReview,
	items []models.ResumeReviewItem,
) (annotatedAvailable bool, err error) {
	if _, annotatedStored, annotateErr := s.annotatePDF(pdfPath, doc, review, items); annotateErr != nil {
		log.Printf("resume_review: annotatePDF failed document_id=%d err=%v", doc.ID, annotateErr)
		doc.AnnotatedPath = ""
	} else {
		doc.AnnotatedPath = annotatedStored
		annotatedAvailable = true
	}

	doc.Status = "reviewed"
	if err := s.repo.UpdateDocument(doc); err != nil {
		log.Printf("resume_review: UpdateDocument failed document_id=%d err=%v", doc.ID, err)
		return annotatedAvailable, err
	}
	return annotatedAvailable, nil
}

// persistReview はレビュー本体・指摘項目・スコア連携をまとめて保存する。
//
// 通常経路(ReviewDocument)とストリーム経路(ReviewDocumentStream)の両方から呼ぶ。
// 以前はこの一連がストリーム側に無く、フロントが使っているのはストリーム側
// だったため、画面にはレビューが出るのに resume_reviews は0件のままだった(#1332)。
// 学生は画面を閉じるとレビューを二度と見られず、スコアが無いので
// EvaluateResumeStatus は永久に「要対応」と判定し、履歴書スコアが
// user_weight_scores に載らずフライホイールも回っていなかった。
//
// 二度と片側だけ育たないよう、保存はここ1箇所に集約する。
func (s *ResumeService) persistReview(doc *models.ResumeDocument, review *models.ResumeReview, items []models.ResumeReviewItem) error {
	review.DocumentID = doc.ID
	if err := s.repo.CreateReview(review); err != nil {
		return err
	}
	for i := range items {
		items[i].ReviewID = review.ID
	}
	if err := s.repo.ReplaceReviewItems(review.ID, items); err != nil {
		return err
	}

	// スコア連携の失敗はレビュー保存を巻き戻すほどではない。ログに残して続ける。
	if s.crossFeature != nil {
		if err := s.crossFeature.UpdateScoresFromResumeReview(doc.UserID, doc.SessionID, review, items); err != nil {
			log.Printf("[Resume] crossFeature score update failed: %v\n", err)
		}
	}
	return nil
}

func (s *ResumeService) ReviewDocumentStream(ctx context.Context, documentID uint, requestingUserID uint, companyName, jobTitle, candidateType string, w http.ResponseWriter) error {
	sendEvent := func(v map[string]any) {
		data, _ := json.Marshal(v)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flushSSE(w)
	}

	doc, err := s.repo.FindDocumentByIDForUser(documentID, requestingUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sendEvent(map[string]any{"type": "error", "message": "forbidden"})
			return shared.ErrForbidden
		}
		sendEvent(map[string]any{"type": "error", "message": err.Error()})
		return err
	}
	if strings.TrimSpace(companyName) == "" && strings.TrimSpace(jobTitle) == "" {
		msg := "応募企業名または応募職種を入力してください"
		sendEvent(map[string]any{"type": "error", "message": msg})
		return errors.New(msg)
	}
	// 未登録企業の取得は最大 provisionTimeout かかる。その間レスポンスが
	// 無言だとエッジ(60秒)に切られるため、先に1イベント流して時計を進めておく。
	sendEvent(map[string]any{"type": "progress", "message": "企業情報を確認しています"})

	canonicalName, err := s.ensureRealCompany(ctx, companyName)
	if err != nil {
		msg := err.Error()
		var ve *shared.ValidationError
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
		log.Printf("resume_review_stream: pdf path outside workDir pdf=%q workDir=%q err=%v", pdfPath, workDir, err)
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
			ragReport, _ = relaySSEChunks(ragBody, w)
			ragBody.Close()
		} else {
			log.Printf("resume_review_stream: rag stream failed: %v", ragErr)
		}
	}

	// スコア・指摘事項を生成
	review, items, err := s.buildReviewScoreItems(blocks, companyName, jobTitle, candidateType, ragReport)
	if err != nil {
		log.Printf("resume_review_stream: build score failed: %v", err)
		var ve *shared.ValidationError
		switch {
		case errors.As(err, &ve):
			sendEvent(map[string]any{"type": "error", "message": ve.Message})
		case errors.Is(err, openai.ErrAIUnavailable):
			// 内部設定が学生に見えないよう固定文言にする(#1293)
			sendEvent(map[string]any{"type": "error", "message": "現在AI機能を利用できません。しばらくしてから再度お試しください。"})
		default:
			sendEvent(map[string]any{"type": "error", "message": err.Error()})
		}
		return err
	}

	// レビューを保存する。注釈PDFより先に行う。
	// ここが欠けていたため、画面には出るのに resume_reviews が0件のままだった(#1332)。
	if err := s.persistReview(doc, review, items); err != nil {
		log.Printf("resume_review_stream: persistReview failed document_id=%d err=%v", documentID, err)
		sendEvent(map[string]any{"type": "error", "message": "レビュー結果の保存に失敗しました。もう一度お試しください。"})
		return err
	}

	annotatedAvailable, err := s.finalizeDocument(doc, pdfPath, review, items)
	if !annotatedAvailable {
		sendEvent(map[string]any{
			"type":    "annotate_error",
			"message": "注釈PDFの生成に失敗しました。レビュー結果は表示されますが、PDFダウンロードはご利用いただけません。",
		})
	}
	if err != nil {
		// ここを握り潰すと「レビューはあるのに要対応のまま」になる。
		sendEvent(map[string]any{"type": "error", "message": "レビュー結果の保存に失敗しました。もう一度お試しください。"})
		return err
	}

	sendEvent(map[string]any{
		"type":                "complete",
		"review":              review,
		"items":               items,
		"annotated_available": annotatedAvailable,
	})
	return nil
}

// flushSSE はバッファを送出する。Echo の Response.Flush は WrapMiddleware 配下で
// panic するため使わず、Unwrap して実体の Flusher を探す (#855)。
func flushSSE(w http.ResponseWriter) {
	rw := w
	for range 8 {
		u, unwraps := rw.(interface{ Unwrap() http.ResponseWriter })
		if unwraps {
			if next := u.Unwrap(); next != nil && next != rw {
				rw = next
				continue
			}
		}
		if f, ok := rw.(http.Flusher); ok {
			f.Flush()
		}
		return
	}
}

// relaySSEChunks はFastAPIからのSSEストリームを読み取り、chunk イベントをそのまま
// クライアントに転送しつつ、全テキストを返す。
func relaySSEChunks(body io.Reader, w http.ResponseWriter) (string, error) {
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
			flushSSE(w)
		case "done":
			return accum.String(), nil
		case "error":
			return accum.String(), fmt.Errorf("rag stream error: %s", event.Message)
		}
	}
	return accum.String(), scanner.Err()
}

func (s *ResumeService) buildResumeReviewWithAI(ctx context.Context, blocks []models.ResumeTextBlock, companyName string, jobTitle string, candidateType string) (*models.ResumeReview, []models.ResumeReviewItem, error) {
	text := buildResumeText(blocks, 30000)
	if strings.TrimSpace(text) == "" {
		return nil, nil, &shared.ValidationError{Message: "履歴書からテキストを抽出できませんでした。PDF の画質や形式を確認してください"}
	}
	if s.aiClient == nil {
		return nil, nil, fmt.Errorf("AIクライアントが初期化されていません")
	}

	var companyInfo string
	if strings.TrimSpace(companyName) != "" {
		if ragReport, err := s.fetchRAGReport(ctx, text, companyName, jobTitle); err == nil {
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
	// 企業briefは RAGレポートの有無に関わらず必ず併記する（#1124）。
	//
	// 「重視傾向」を出力するのは brief だけで、RAGレポートには含まれない。
	// 以前は RAGレポートが空のときしか brief を使っていなかったため、
	// プロンプト側の「重視傾向があれば不足を指摘してよい」という条件が
	// 通常経路では一度も成立していなかった。
	if strings.TrimSpace(companyName) != "" {
		if brief := s.lookupCompanyBriefFromCache(companyName); brief != "" {
			if strings.TrimSpace(companyInfo) == "" {
				companyInfo = brief
			} else {
				companyInfo = brief + "\n\n" + companyInfo
			}
		}
	}

	prompt := fmt.Sprintf(`以下は履歴書/エントリーシートのOCRテキストです。
この内容をレビューし、改善すべき点を最大8件までJSONで返してください。
必ず本文中に存在する短い引用(quote)を入れてください。quoteは後で位置合わせに使います。

原則として、本文の内容に基づいた具体的な改善点のみを書いてください。
「記載されていません」「未記入」といった欠落の指摘は、次の条件を**すべて**満たす場合にだけ
許可します。それ以外の欠落指摘は禁止です。
  (1) 「企業情報(参考)」に「重視傾向:」の行があり、
  (2) 指摘する内容がその重視傾向に挙がっている軸に対応していて、
  (3) quote に本文の実在するブロックを選び、
  (4) suggestion に「その軸を裏付けるには、この記述に何を足せばよいか」を具体的に書く
（例: 重視傾向がリーダーシップなら、既存の活動記述に役割・人数・期間・成果を足す案を出す）。
「重視傾向:」の行が無い場合は、欠落の指摘を一切しないでください。
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
「企業情報(参考)」に重視傾向がある場合は、その軸を優先的に扱ってください。
企業情報が空、または重視傾向が無い場合は、一般的な観点でレビューしてください
（存在しない企業の特徴を推測して書かないこと）。

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
