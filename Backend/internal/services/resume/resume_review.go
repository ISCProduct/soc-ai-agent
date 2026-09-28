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

// 評価ハーネス（cmd/aibench）が本番と同じ指示・同じ呼び出し条件で測れるように、
// プロンプトと呼び出しパラメータを公開する（#1525）。
// ハーネス側に写すと、本文やパラメータを直した瞬間に測っている対象が本番と
// 別物になり、出た数字が判断材料として使えなくなる。
const (
	// ReviewSystemPrompt はレビュー生成の system プロンプト。
	ReviewSystemPrompt = "あなたは日本語の履歴書・エントリーシートを添削する専門家です。必ず具体的な書き換え案をJSON形式で提示します。"

	// ReviewMaxOutputTokens はレビュー生成の出力上限。
	//
	// 指摘8件（引用・指摘・改善案の3文×8）が出力の大半で、#1529 で追加した
	// scores 5項目は数十トークン程度。実測で8件のとき約1310〜1360トークンなので
	// 1.8倍程度の余裕がある。上限に達したときは requestReviewJSON が枠を倍にして
	// 1度だけやり直す。
	ReviewMaxOutputTokens = 2400

	// ReviewTemperature はレビュー生成の温度。
	ReviewTemperature = 0.2

	// ReviewMaxItems はプロンプトが指定している items の上限。
	// プロンプト本文と指示遵守率の判定で同じ値を使う。
	ReviewMaxItems = 8

	// reviewTextLimit はプロンプトへ載せるOCRテキストの上限バイト数。
	reviewTextLimit = 30000
)

// ReviewModel はレビューに使うモデル名を返す（env 未設定なら既定）。
func ReviewModel() string {
	if m := strings.TrimSpace(os.Getenv("OPENAI_REVIEW_MODEL")); m != "" {
		return m
	}
	return "gpt-4o-mini"
}

// errReviewOutputTruncated は出力上限に達して JSON が完結しなかったことを表す。
//
// ValidationError にしてあるのは、コントローラが 422 に写すため。
// #1521 の ES 添削（docs/wiki/rag-service.md）も再試行後の上限到達は 422 で返す。
// 文言はそのまま学生に見えるため、内部事情は書かない。
var errReviewOutputTruncated error = &shared.ValidationError{
	Message: "AIレビューの出力が長すぎて途中で切れました。再度お試しください",
}

// requestReviewJSON はレビュー用の JSON を取得する。
// 出力上限に達したら枠を倍にして1度だけやり直し、それでも切れたら
// errReviewOutputTruncated を返す（#1521 の定石。docs/wiki/rag-service.md）。
//
// クライアント内部にも枠を倍にする再試行はあるが、発火条件は
// 「本文が空で、エラー文に max_output_tokens が含まれる」ときだけで、
// 本文が途中まで返っているときは発火しない。そこがここの担当分である。
//
// 切れた本文を decodeJSON へ渡さないのは、現在のスキーマでは items が最後の
// フィールドなので必ず解析エラーになるものの、**エラー文言が
// 「解析に失敗しました」になって原因が上限だと分からない**ため。
// items を最後以外へ動かすと途中までが読めてしまう余地も残るので、
// スキーマ変更に対する保険も兼ねている。
func (s *ResumeService) requestReviewJSON(systemPrompt, userPrompt, model string) (string, error) {
	// 同じ枠・同じ温度でやり直しても同じ位置で切れるので、枠を倍にする方だけ試す。
	for _, maxTokens := range []int{ReviewMaxOutputTokens, ReviewMaxOutputTokens * 2} {
		// 上限到達を検知するためフラグ付きのコンテキストで呼ぶ（#1529）。
		aiCtx := openai.WithTruncationFlag(context.Background())
		raw, err := s.aiClient.ResponsesWithMaxTokens(aiCtx, systemPrompt, userPrompt, ReviewTemperature, maxTokens, model)
		if err != nil {
			return "", err
		}
		if !openai.OutputTruncated(aiCtx) {
			return raw, nil
		}
		log.Printf("resume_review: 出力が max_output_tokens=%d に達して切れた（解析しない）", maxTokens)
	}
	return "", errReviewOutputTruncated
}

type aiReviewResponse struct {
	// Scores はルーブリックの項目別スコア（各0〜5）。総合スコアは
	// ComputeResumeOverallScore でサーバー側が算出するため LLM には出させない（#1529）。
	Scores         map[string]int `json:"scores"`
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

	text := buildResumeText(blocks, reviewTextLimit)

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
	text := buildResumeText(blocks, reviewTextLimit)
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
	text := buildResumeText(blocks, reviewTextLimit)
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

	prompt := buildReviewPrompt(text, companyName, jobTitle, companyInfo, candidateType)

	modelOverride := ReviewModel()
	raw, err := s.requestReviewJSON(ReviewSystemPrompt, prompt, modelOverride)
	if err != nil {
		log.Printf("resume_review: openai review failed: %v", err)
		if errors.Is(err, errReviewOutputTruncated) {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("AIレビューの生成に失敗しました。しばらく待ってから再度お試しください")
	}

	response := aiReviewResponse{}
	if err := decodeJSON(raw, &response); err != nil {
		log.Printf("resume_review: decode failed: %v", err)
		return nil, nil, fmt.Errorf("AIレビュー結果の解析に失敗しました。再度お試しください")
	}

	if response.Summary == "" {
		response.Summary = "内容を確認しました。具体性と成果の明確化が改善ポイントです。"
	}

	// スコアはルーブリック検証を通ったものだけ採用する。違反したら固定値を入れず
	// スコア無しにする（docs/wiki/scoring.md §2-3 / §2-5）。
	overall, scoreErr := ComputeResumeOverallScore(response.Scores, candidateType)
	if scoreErr != nil {
		log.Printf("resume_review: ルーブリック違反のためスコアを捨てる: %v", scoreErr)
	}

	items := mapReviewItems(blocks, response.Items)
	log.Printf("resume_review: items mapped=%d raw=%d", len(items), len(response.Items))
	// スコアだけが不正なときもやり直す。面接（#795）と同じく作り直しは1度だけ。
	if len(items) < 3 || scoreErr != nil {
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

%s
scoresは上の評価基準の全項目を必ず含めてください。総合点は出力しないでください（サーバー側で算出します）。

出力は次のJSONのみ:
{"scores":{%s},"summary":"短い要約","items":[{"quote":"本文中の一文","message":"指摘","suggestion":"改善案","severity":"info|warning|critical","page_hint":1,"block_index":1}]}`,
			companyName, jobTitle, companyInfo, candidateType, blockList,
			BuildResumeRubricPromptSection(), buildRubricJSONHint())
		rawRetry, retryReqErr := s.requestReviewJSON("あなたは日本語の履歴書・エントリーシートを添削する専門家です。JSON形式で出力してください。", retryPrompt, modelOverride)
		if retryReqErr != nil {
			log.Printf("resume_review: やり直しの生成に失敗: %v", retryReqErr)
		}
		if retryReqErr == nil {
			responseRetry := aiReviewResponse{}
			if decodeJSON(rawRetry, &responseRetry) == nil {
				retryItems := mapReviewItems(blocks, responseRetry.Items)
				log.Printf("resume_review: retry items mapped=%d raw=%d", len(retryItems), len(responseRetry.Items))
				// 指摘の差し替えは「初回が3件未満」のときだけ。スコアだけが不正で
				// 初回の指摘が足りているなら、本文に紐づいた初回の指摘を守る。
				// やり直しはブロック一覧ベースの別プロンプトで、件数は増えても
				// 紐づきの質が上がる保証が無い（採点のやり直しに指摘を賭けない）。
				if len(items) < 3 {
					items = adoptRetryItems(items, retryItems)
				}
				// スコアが不正だったときだけ差し替える。初回が正当なら上書きしない
				// （やり直しは指摘の紐づけを増やすためのもので、採点のやり直しではない）。
				if scoreErr != nil {
					if retryScore, retryErr := ComputeResumeOverallScore(responseRetry.Scores, candidateType); retryErr == nil {
						overall, scoreErr = retryScore, nil
					} else {
						log.Printf("resume_review: やり直してもルーブリック違反: %v", retryErr)
					}
				}
			}
		}
	}
	if len(items) == 0 {
		log.Println("resume_review: no items could be mapped after retry")
		return nil, nil, fmt.Errorf("AIが生成した指摘内容を履歴書ブロックに紐づけられませんでした。再度お試しください")
	}

	review := &models.ResumeReview{Summary: response.Summary}
	// スコア無しのときは Score / ItemScoresJSON をどちらも nil のままにする。
	// 片方だけ残すと画面の総合スコアと内訳が食い違う。
	if scoreErr == nil {
		encoded, err := json.Marshal(response.Scores)
		if err != nil {
			// map[string]int なので実際には起きない。起きたら内訳だけ落とす。
			log.Printf("resume_review: 項目スコアのJSON化に失敗: %v", err)
		} else {
			itemScores := string(encoded)
			review.ItemScoresJSON = &itemScores
		}
		review.Score = &overall
		log.Printf("resume_review: 総合スコア=%d 候補者区分=%q 内訳=%v", overall, candidateType, response.Scores)
	}
	return review, items, nil
}

// buildRubricJSONHint はプロンプトの出力例に入れる scores のキー列を作る。
// 例を手書きすると評価項目を増減したときに食い違うため、定義から作る。
func buildRubricJSONHint() string {
	keys := ResumeRubricKeys()
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%q:0-%d", key, ResumeRubricScoreMax))
	}
	return strings.Join(parts, ",")
}

// buildReviewPrompt は履歴書レビューの user プロンプトを組み立てる。
// text は buildResumeText が付ける [P#B#] 付きのOCRテキスト。
//
// buildReviewScoreItems から切り出してあるのは、評価ハーネス（cmd/aibench）が
// DB・S3・RAG を用意せずに**本番と同一のプロンプト**を評価できるようにするため（#1525）。
func buildReviewPrompt(text, companyName, jobTitle, companyInfo, candidateType string) string {
	return fmt.Sprintf(`以下は履歴書/エントリーシートのOCRテキストです。
この内容をレビューし、改善すべき点を最大%d件までJSONで返してください。
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

%s
scoresは上の評価基準の全項目を必ず含めてください。総合点は出力しないでください（サーバー側で算出します）。

出力は次のJSONのみ:
{"scores":{%s},"summary":"短い要約","items":[{"quote":"本文中の一文","message":"指摘","suggestion":"改善案","severity":"info|warning|critical","page_hint":1,"block_index":1}]}

OCRテキスト:
%s`, ReviewMaxItems, companyName, jobTitle, companyInfo, candidateType,
		BuildResumeRubricPromptSection(), buildRubricJSONHint(), text)
}

// BuildReviewPromptFromText は行区切りの平文から本番と同一の user プロンプトを作る。
//
// 評価ハーネス用の入口。ゴールデンセットは OCR 結果ではなく平文で持つため、
// 本番の [P#B#] 付きテキストへ同じ手順で変換してからプロンプトへ載せる。
// 1行=1ブロック（ページは1固定）として扱う。
func BuildReviewPromptFromText(resumeText, companyName, jobTitle, companyInfo, candidateType string) string {
	blocks := make([]models.ResumeTextBlock, 0, 16)
	for _, line := range strings.Split(resumeText, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		blocks = append(blocks, models.ResumeTextBlock{
			PageNumber: 1,
			BlockIndex: len(blocks),
			Text:       line,
		})
	}
	return buildReviewPrompt(buildResumeText(blocks, reviewTextLimit), companyName, jobTitle, companyInfo, candidateType)
}
