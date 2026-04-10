package services

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ValidationError はユーザー入力起因のエラーを表す。controller で 422 を返すために使用する。
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type ResumeService struct {
	repo         repository.ResumeRepository
	storageDir   string
	aiClient     *openai.Client
	s3           *s3Storage
	s3Err        error
	crossFeature *CrossFeatureIntegrationService
}

// SetCrossFeatureService 機能間連携サービスを注入する（オプション）
func (s *ResumeService) SetCrossFeatureService(cf *CrossFeatureIntegrationService) {
	s.crossFeature = cf
}

func NewResumeService(repo repository.ResumeRepository, storageDir string, aiClient *openai.Client) *ResumeService {
	if strings.TrimSpace(storageDir) == "" {
		storageDir = "storage/resumes"
	}
	s3Store, s3Err := newS3StorageFromEnv(context.Background())
	return &ResumeService{
		repo:       repo,
		storageDir: storageDir,
		aiClient:   aiClient,
		s3:         s3Store,
		s3Err:      s3Err,
	}
}

type ResumeUploadResult struct {
	Document *models.ResumeDocument `json:"document"`
}

func (s *ResumeService) Upload(userID uint, sessionID, sourceType, sourceURL string, fileHeader *multipart.FileHeader) (*ResumeUploadResult, error) {
	if strings.TrimSpace(sourceType) == "" {
		sourceType = "pdf"
	}
	if fileHeader == nil && strings.TrimSpace(sourceURL) == "" {
		return nil, errors.New("file or source_url is required")
	}
	if err := s.ensureS3Available(); err != nil {
		return nil, err
	}
	if s.s3 == nil || !s.s3.isEnabled() {
		return nil, errors.New("s3 is required")
	}

	doc := &models.ResumeDocument{
		UserID:     userID,
		SessionID:  sessionID,
		SourceType: sourceType,
		SourceURL:  sourceURL,
		Status:     "uploaded",
	}

	workDir, err := os.MkdirTemp("", fmt.Sprintf("resume_upload_%d_", userID))
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	if fileHeader != nil {
		doc.OriginalFilename = fileHeader.Filename
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		originalPath := filepath.Join(workDir, "original"+ext)
		if err := saveUploadedFile(fileHeader, originalPath); err != nil {
			return nil, err
		}
		doc.StoredPath = originalPath
	} else if strings.ToLower(sourceType) == "google_docs" {
		doc.OriginalFilename = "google_doc"
		doc.StoredPath = ""
	} else {
		downloaded, filename, err := downloadSourceFile(sourceURL, workDir)
		if err != nil {
			return nil, err
		}
		doc.OriginalFilename = filename
		doc.StoredPath = downloaded
	}

	if err := s.repo.CreateDocument(doc); err != nil {
		return nil, fmt.Errorf("failed to save document: %w", err)
	}
	if doc.StoredPath != "" {
		filename := filepath.Base(doc.StoredPath)
		s3Path, err := s.uploadToS3(context.Background(), doc, doc.StoredPath, filename)
		if err != nil {
			return nil, err
		}
		doc.StoredPath = s3Path
		if err := s.repo.UpdateDocument(doc); err != nil {
			return nil, fmt.Errorf("failed to update document: %w", err)
		}
	}

	return &ResumeUploadResult{Document: doc}, nil
}

func (s *ResumeService) ReviewDocument(documentID uint, companyName string, jobTitle string, candidateType string) (*models.ResumeReview, []models.ResumeReviewItem, error) {
	doc, err := s.repo.FindDocumentByID(documentID)
	if err != nil {
		return nil, nil, err
	}
	if s.s3 == nil || !s.s3.isEnabled() {
		return nil, nil, errors.New("s3 is required")
	}
	if strings.TrimSpace(companyName) == "" && strings.TrimSpace(jobTitle) == "" {
		return nil, nil, &ValidationError{Message: "応募企業名または応募職種を入力してください"}
	}

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
			fmt.Printf("[Resume] crossFeature score update failed: %v\n", err)
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

type AnnotatedFile struct {
	Reader      io.ReadSeeker
	Size        int64
	ContentType string
	Filename    string
	CloseFunc   func() error
}

func (s *ResumeService) OpenAnnotatedFile(documentID uint) (*AnnotatedFile, error) {
	doc, err := s.repo.FindDocumentByID(documentID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(doc.AnnotatedPath) == "" {
		return nil, errors.New("annotated file not ready")
	}
	if !isS3URI(doc.AnnotatedPath) {
		f, err := os.Open(doc.AnnotatedPath)
		if err != nil {
			return nil, err
		}
		stat, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, err
		}
		return &AnnotatedFile{
			Reader:      f,
			Size:        stat.Size(),
			ContentType: "application/pdf",
			Filename:    filepath.Base(doc.AnnotatedPath),
			CloseFunc:   f.Close,
		}, nil
	}
	if err := s.ensureS3Available(); err != nil {
		return nil, err
	}
	bucket, key, ok := parseS3URI(doc.AnnotatedPath)
	if !ok {
		return nil, errors.New("invalid s3 path")
	}
	if bucket != s.s3.bucket {
		return nil, errors.New("s3 bucket mismatch")
	}
	resp, err := s.s3.getObject(context.Background(), key)
	if err != nil {
		return nil, err
	}
	contentType := "application/pdf"
	if resp.ContentType != nil && strings.TrimSpace(*resp.ContentType) != "" {
		contentType = *resp.ContentType
	}
	reader := newSeekableReader(resp.Body)
	return &AnnotatedFile{
		Reader:      reader,
		Size:        derefInt64(resp.ContentLength),
		ContentType: contentType,
		Filename:    filepath.Base(key),
		CloseFunc:   reader.Close,
	}, nil
}

func (s *ResumeService) normalizeToPDF(doc *models.ResumeDocument, workDir string) (string, string, error) {
	if doc.StoredPath == "" {
		return "", "", errors.New("document path not found")
	}
	localPath, err := s.resolveLocalPath(doc, workDir)
	if err != nil {
		return "", "", err
	}
	ext := strings.ToLower(filepath.Ext(localPath))
	if ext == ".pdf" {
		storedPath, err := s.uploadToS3(context.Background(), doc, localPath, "normalized.pdf")
		if err != nil {
			return "", "", err
		}
		return localPath, storedPath, nil
	}

	outputDir := filepath.Dir(localPath)
	cmd := exec.Command("soffice", "--headless", "--convert-to", "pdf", "--outdir", outputDir, localPath)
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to convert to pdf: %w", err)
	}

	pdfPath := strings.TrimSuffix(localPath, ext) + ".pdf"
	if _, err := os.Stat(pdfPath); err != nil {
		return "", "", fmt.Errorf("converted pdf not found: %w", err)
	}
	storedPath, err := s.uploadToS3(context.Background(), doc, pdfPath, "normalized.pdf")
	if err != nil {
		return "", "", err
	}
	return pdfPath, storedPath, nil
}

type ocrPayload struct {
	Pages []struct {
		PageNumber int `json:"page_number"`
		Width      int `json:"width"`
		Height     int `json:"height"`
		Blocks     []struct {
			BlockIndex int       `json:"block_index"`
			Text       string    `json:"text"`
			BBox       []float64 `json:"bbox"`
		} `json:"blocks"`
	} `json:"pages"`
}

func (s *ResumeService) extractTextBlocks(doc *models.ResumeDocument, pdfPath string) ([]models.ResumeTextBlock, error) {
	scriptPath := filepath.Join("scripts", "ocr_extract.py")
	cmd := exec.Command("python3", scriptPath, "--input", pdfPath)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// stderr（スタックトレース含む）はサーバー側ログにのみ記録し、クライアントには漏らさない
		log.Printf("ocr script error: %v\nstderr:\n%s", err, stderr.String())
		return nil, fmt.Errorf("ocr failed: %w", err)
	}

	var payload ocrPayload
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		return nil, fmt.Errorf("failed to parse ocr result: %w", err)
	}

	var blocks []models.ResumeTextBlock
	for _, page := range payload.Pages {
		for _, block := range page.Blocks {
			bbox, _ := json.Marshal(map[string]interface{}{
				"bbox":        block.BBox,
				"page_width":  page.Width,
				"page_height": page.Height,
			})
			blocks = append(blocks, models.ResumeTextBlock{
				DocumentID: doc.ID,
				PageNumber: page.PageNumber,
				BlockIndex: block.BlockIndex,
				Text:       block.Text,
				BBox:       string(bbox),
			})
		}
	}
	return blocks, nil
}

func (s *ResumeService) annotatePDF(inputPath string, doc *models.ResumeDocument, review *models.ResumeReview, items []models.ResumeReviewItem) (string, string, error) {
	if len(items) == 0 {
		storedPath, err := s.uploadToS3(context.Background(), doc, inputPath, "annotated.pdf")
		if err != nil {
			return "", "", err
		}
		return inputPath, storedPath, nil
	}
	payload := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		bboxInfo := decodeBBoxInfo(item.BBox)
		payload = append(payload, map[string]interface{}{
			"page_number": item.PageNumber,
			"bbox":        bboxInfo.BBox,
			"page_width":  bboxInfo.PageWidth,
			"page_height": bboxInfo.PageHeight,
			"severity":    item.Severity,
			"message":     item.Message,
			"suggestion":  item.Suggestion,
		})
	}

	itemsPath := filepath.Join(filepath.Dir(inputPath), fmt.Sprintf("review_items_%d.json", review.ID))
	data, _ := json.Marshal(payload)
	if err := os.WriteFile(itemsPath, data, 0o644); err != nil {
		return "", "", err
	}

	if err := copyFile(inputPath, filepath.Join(filepath.Dir(inputPath), "original_copy.pdf")); err != nil {
		return "", "", err
	}
	if s.s3.isEnabled() {
		_, err := s.uploadToS3(context.Background(), doc, filepath.Join(filepath.Dir(inputPath), "original_copy.pdf"), "original_copy.pdf")
		if err != nil {
			return "", "", err
		}
	}

	outputPath := filepath.Join(filepath.Dir(inputPath), "annotated.pdf")
	scriptPath := filepath.Join("scripts", "annotate_pdf.py")
	cmd := exec.Command("python3", scriptPath, "--input", inputPath, "--output", outputPath, "--items", itemsPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("annotate failed: %w (%s)", err, stderr.String())
	}
	storedPath, err := s.uploadToS3(context.Background(), doc, outputPath, filepath.Base(outputPath))
	if err != nil {
		return "", "", err
	}
	return outputPath, storedPath, nil
}

type bboxInfo struct {
	BBox       []float64
	PageWidth  float64
	PageHeight float64
}

func decodeBBoxInfo(raw string) bboxInfo {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return bboxInfo{}
	}
	var bbox []float64
	if err := json.Unmarshal([]byte(raw), &bbox); err == nil {
		return bboxInfo{BBox: bbox}
	}
	var payload struct {
		BBox       []float64 `json:"bbox"`
		PageWidth  float64   `json:"page_width"`
		PageHeight float64   `json:"page_height"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err == nil {
		return bboxInfo{
			BBox:       payload.BBox,
			PageWidth:  payload.PageWidth,
			PageHeight: payload.PageHeight,
		}
	}
	return bboxInfo{}
}

func (s *ResumeService) ensureS3Available() error {
	if s.s3Err != nil {
		return s.s3Err
	}
	return nil
}

func isS3URI(path string) bool {
	return strings.HasPrefix(path, "s3://")
}

func (s *ResumeService) ensureWorkingDir(docID uint) (string, error) {
	return os.MkdirTemp("", fmt.Sprintf("resume_work_%d_", docID))
}

func (s *ResumeService) resolveLocalPath(doc *models.ResumeDocument, workDir string) (string, error) {
	if !isS3URI(doc.StoredPath) {
		if strings.TrimSpace(doc.StoredPath) == "" && strings.TrimSpace(doc.SourceURL) != "" {
			downloaded, _, err := downloadSourceFile(doc.SourceURL, workDir)
			if err != nil {
				return "", err
			}
			return downloaded, nil
		}
		return doc.StoredPath, nil
	}
	if err := s.ensureS3Available(); err != nil {
		return "", err
	}
	if s.s3 == nil || !s.s3.isEnabled() {
		return "", errors.New("s3 is not configured")
	}
	bucket, key, ok := parseS3URI(doc.StoredPath)
	if !ok {
		return "", errors.New("invalid s3 path")
	}
	if bucket != s.s3.bucket {
		return "", errors.New("s3 bucket mismatch")
	}
	ext := strings.ToLower(filepath.Ext(doc.OriginalFilename))
	if ext == "" {
		ext = ".pdf"
	}
	dest := filepath.Join(workDir, "original"+ext)
	if err := s.s3.downloadToFile(context.Background(), key, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func (s *ResumeService) s3KeyForDocument(doc *models.ResumeDocument, filename string) string {
	return s.s3.objectKey("resumes", fmt.Sprintf("%d", doc.UserID), fmt.Sprintf("%d", doc.ID), filename)
}

func (s *ResumeService) uploadToS3(ctx context.Context, doc *models.ResumeDocument, localPath, filename string) (string, error) {
	if s.s3 == nil || !s.s3.isEnabled() {
		return localPath, nil
	}
	if err := s.ensureS3Available(); err != nil {
		return "", err
	}
	key := s.s3KeyForDocument(doc, filename)
	return s.s3.uploadFile(ctx, key, localPath, contentTypeForPath(localPath))
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	return copyToWriter(in, out)
}

type seekableReader struct {
	data []byte
	r    *bytes.Reader
}

func newSeekableReader(src io.ReadCloser) *seekableReader {
	defer src.Close()
	data, _ := io.ReadAll(src)
	return &seekableReader{
		data: data,
		r:    bytes.NewReader(data),
	}
}

func (s *seekableReader) Read(p []byte) (int, error) {
	return s.r.Read(p)
}

func (s *seekableReader) Seek(offset int64, whence int) (int64, error) {
	return s.r.Seek(offset, whence)
}

func (s *seekableReader) Close() error {
	return nil
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func saveUploadedFile(fileHeader *multipart.FileHeader, dest string) error {
	src, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, src); err != nil {
		return err
	}
	return nil
}

func downloadSourceFile(url, storagePath string) (string, string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("download failed: %s", resp.Status)
	}

	filename := "downloaded"
	if disp := resp.Header.Get("Content-Disposition"); disp != "" {
		if parts := strings.Split(disp, "filename="); len(parts) > 1 {
			filename = strings.Trim(parts[1], "\"")
		}
	}
	if filename == "downloaded" {
		filename = filepath.Base(req.URL.Path)
	}
	if filename == "" || filename == "." || filename == "/" {
		filename = "document"
	}
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".pdf"
		filename += ext
	}

	dest := filepath.Join(storagePath, filename)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(dest, body, 0o644); err != nil {
		return "", "", err
	}
	return dest, filename, nil
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
	ResumeText  string `json:"resume_text"`
	CompanyName string `json:"company_name"`
	JobTitle    string `json:"job_title"`
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
		ResumeText:  resumeText,
		CompanyName: companyName,
		JobTitle:    jobTitle,
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
		ResumeText:  resumeText,
		CompanyName: companyName,
		JobTitle:    jobTitle,
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
func (s *ResumeService) ReviewDocumentStream(ctx context.Context, documentID uint, companyName, jobTitle, candidateType string, w http.ResponseWriter) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	sendEvent := func(v map[string]interface{}) {
		data, _ := json.Marshal(v)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	doc, err := s.repo.FindDocumentByID(documentID)
	if err != nil {
		sendEvent(map[string]interface{}{"type": "error", "message": err.Error()})
		return err
	}
	if s.s3 == nil || !s.s3.isEnabled() {
		sendEvent(map[string]interface{}{"type": "error", "message": "s3 is required"})
		return errors.New("s3 is required")
	}
	if strings.TrimSpace(companyName) == "" && strings.TrimSpace(jobTitle) == "" {
		msg := "応募企業名または応募職種を入力してください"
		sendEvent(map[string]interface{}{"type": "error", "message": msg})
		return errors.New(msg)
	}

	workDir, err := s.ensureWorkingDir(doc.ID)
	if err != nil {
		sendEvent(map[string]interface{}{"type": "error", "message": err.Error()})
		return err
	}
	defer os.RemoveAll(workDir)

	pdfPath, normalizedStored, err := s.normalizeToPDF(doc, workDir)
	if err != nil {
		sendEvent(map[string]interface{}{"type": "error", "message": err.Error()})
		return err
	}
	doc.NormalizedPath = normalizedStored
	doc.Status = "normalized"
	_ = s.repo.UpdateDocument(doc)

	blocks, err := s.extractTextBlocks(doc, pdfPath)
	if err != nil {
		sendEvent(map[string]interface{}{"type": "error", "message": err.Error()})
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
		sendEvent(map[string]interface{}{"type": "error", "message": msg})
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
			sendEvent(map[string]interface{}{"type": "error", "message": ve.Message})
		} else {
			sendEvent(map[string]interface{}{"type": "error", "message": err.Error()})
		}
		return err
	}

	// 注釈 PDF を生成して保存
	annotatedAvailable := false
	_, annotatedStored, err := s.annotatePDF(pdfPath, doc, review, items)
	if err != nil {
		log.Printf("resume_review_stream: annotatePDF failed document_id=%d err=%v", documentID, err)
		sendEvent(map[string]interface{}{
			"type":    "annotate_error",
			"message": "注釈PDFの生成に失敗しました。レビュー結果は表示されますが、PDFダウンロードはご利用いただけません。",
		})
	} else {
		doc.AnnotatedPath = annotatedStored
		doc.Status = "reviewed"
		_ = s.repo.UpdateDocument(doc)
		annotatedAvailable = true
	}

	sendEvent(map[string]interface{}{
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
		companyPrompt := fmt.Sprintf(`企業名: %s
採用観点（求める人物像・評価軸・事業領域）を簡潔に整理してください。
不確かな情報は断定せず、一般的に言える範囲で述べてください。
出力は次のJSONのみ:
{"summary":"200〜300字の企業概要","evaluation_axes":["評価軸1","評価軸2"],"keywords":["キーワード1","キーワード2"]}`, companyName)
		info, err := s.aiClient.Responses(context.Background(), companyPrompt)
		if err == nil {
			companyInfo = info
		} else {
			log.Printf("resume_review: company summary failed: %v", err)
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

func fallbackResumeReview(blocks []models.ResumeTextBlock) (*models.ResumeReview, []models.ResumeReviewItem) {
	score := 70
	summary := "主要セクションを確認しました。具体性の強化が改善ポイントです。"
	items := make([]models.ResumeReviewItem, 0)
	bbox, _ := json.Marshal([]float64{20, 20, 260, 80})
	items = append(items, models.ResumeReviewItem{
		PageNumber: 1,
		BBox:       string(bbox),
		Severity:   "info",
		Message:    "内容は整理されていますが、成果の具体性や背景の説明が不足しがちです。",
		Suggestion: "成果を数値で示し、役割や工夫点・課題を一文ずつ補足してください。",
	})
	return &models.ResumeReview{
		Score:   score,
		Summary: summary,
	}, items
}

func fallbackResumeReviewDetailed(blocks []models.ResumeTextBlock) (*models.ResumeReview, []models.ResumeReviewItem) {
	score := 70
	summary := "内容を確認しました。各項目の具体性を高めると説得力が増します。"
	items := buildHeuristicItems(blocks, 8)
	if len(items) == 0 {
		return fallbackResumeReview(blocks)
	}
	return &models.ResumeReview{
		Score:   score,
		Summary: summary,
	}, items
}

func buildResumeText(blocks []models.ResumeTextBlock, maxLen int) string {
	if len(blocks) == 0 {
		return ""
	}
	var b strings.Builder
	for _, block := range blocks {
		line := strings.TrimSpace(block.Text)
		if line == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("[P%dB%d] %s\n", block.PageNumber, block.BlockIndex, line))
		if b.Len() >= maxLen {
			break
		}
	}
	return b.String()
}

func decodeJSON(raw string, out interface{}) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("empty response")
	}
	if err := json.Unmarshal([]byte(raw), out); err == nil {
		return nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return json.Unmarshal([]byte(raw[start:end+1]), out)
	}
	return errors.New("invalid JSON response")
}

func mapReviewItems(blocks []models.ResumeTextBlock, aiItems []aiReviewItem) []models.ResumeReviewItem {
	if len(aiItems) == 0 {
		return nil
	}
	result := make([]models.ResumeReviewItem, 0, len(aiItems))
	for _, item := range aiItems {
		if strings.TrimSpace(item.Quote) == "" {
			continue
		}
		var block *models.ResumeTextBlock
		foundByIndex := false
		if item.PageHint > 0 && item.BlockIndex > 0 {
			block = findBlockByIndex(blocks, item.PageHint, item.BlockIndex)
			if block != nil {
				foundByIndex = true
			}
		}
		if block == nil && runeLen(item.Quote) >= 6 {
			block = findBestBlock(blocks, item.Quote, item.PageHint)
		}
		if block == nil {
			continue
		}
		if !foundByIndex && !quoteInBlock(item.Quote, block.Text) {
			continue
		}
		severity := strings.ToLower(item.Severity)
		if severity == "" {
			severity = "info"
		}
		result = append(result, models.ResumeReviewItem{
			PageNumber: block.PageNumber,
			BBox:       block.BBox,
			Severity:   severity,
			Message:    item.Message,
			Suggestion: item.Suggestion,
		})
	}
	return result
}

func findBestBlock(blocks []models.ResumeTextBlock, quote string, pageHint int) *models.ResumeTextBlock {
	quoteNorm := normalizeText(quote)
	if quoteNorm == "" {
		return nil
	}

	var best *models.ResumeTextBlock
	bestScore := 0
	for i := range blocks {
		block := &blocks[i]
		if pageHint > 0 && block.PageNumber != pageHint {
			continue
		}
		blockNorm := normalizeText(block.Text)
		if blockNorm == "" {
			continue
		}
		score := textMatchScore(blockNorm, quoteNorm)
		if score > bestScore {
			bestScore = score
			best = block
		}
	}

	if best != nil {
		return best
	}

	for i := range blocks {
		block := &blocks[i]
		blockNorm := normalizeText(block.Text)
		score := textMatchScore(blockNorm, quoteNorm)
		if score > bestScore {
			bestScore = score
			best = block
		}
	}
	return best
}

func findBlockByIndex(blocks []models.ResumeTextBlock, pageHint int, blockIndex int) *models.ResumeTextBlock {
	for i := range blocks {
		block := &blocks[i]
		if block.PageNumber == pageHint && block.BlockIndex == blockIndex {
			return block
		}
	}
	return nil
}

func normalizeText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "　", "")
	return s
}

func quoteInBlock(quote string, blockText string) bool {
	return strings.Contains(normalizeText(blockText), normalizeText(quote))
}

func runeLen(s string) int {
	return len([]rune(strings.TrimSpace(s)))
}

func buildHeuristicItems(blocks []models.ResumeTextBlock, max int) []models.ResumeReviewItem {
	if len(blocks) == 0 || max <= 0 {
		return nil
	}
	result := make([]models.ResumeReviewItem, 0, max)
	seen := make(map[string]bool)
	for _, block := range blocks {
		text := strings.TrimSpace(block.Text)
		if runeLen(text) < 12 {
			continue
		}
		label, message, suggestion := classifyBlock(text)
		if label == "" {
			continue
		}
		key := fmt.Sprintf("%d-%d-%s", block.PageNumber, block.BlockIndex, label)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, models.ResumeReviewItem{
			PageNumber: block.PageNumber,
			BBox:       block.BBox,
			Severity:   "info",
			Message:    message,
			Suggestion: suggestion,
		})
		if len(result) >= max {
			return result
		}
	}
	if len(result) == 0 {
		for _, block := range blocks {
			text := strings.TrimSpace(block.Text)
			if runeLen(text) < 16 {
				continue
			}
			result = append(result, models.ResumeReviewItem{
				PageNumber: block.PageNumber,
				BBox:       block.BBox,
				Severity:   "info",
				Message:    "この記述は成果や役割の具体性が読み取りづらいです。",
				Suggestion: "成果の数値や担当範囲、工夫点を一文ずつ補足してください。",
			})
			if len(result) >= max {
				return result
			}
		}
	}
	return result
}

func classifyBlock(text string) (string, string, string) {
	switch {
	case strings.Contains(text, "志望") || strings.Contains(text, "動機"):
		return "motivation",
			"志望動機の根拠が抽象的に見えます。",
			"企業の事業や職種と自分の経験の接点を1文で明示し、具体的な業務貢献を追記してください。"
	case strings.Contains(text, "自己PR") || strings.Contains(text, "自己ＰＲ"):
		return "pr",
			"自己PRが強みの列挙にとどまっています。",
			"成果の数値、工夫した点、再現性が分かる行動を1文ずつ追加してください。"
	case strings.Contains(text, "学歴"):
		return "", "", ""
	case strings.Contains(text, "職歴"):
		return "", "", ""
	case strings.Contains(text, "資格") || strings.Contains(text, "免許"):
		return "license",
			"資格が応募職種にどう活かせるかが伝わりづらいです。",
			"資格で得たスキルと職務での活用例を一文追加してください。"
	case strings.Contains(text, "得意") || strings.Contains(text, "特技") || strings.Contains(text, "スキル"):
		return "skill",
			"スキルの記載が抽象的で実務イメージが湧きにくいです。",
			"使用期間、具体的な成果物、担当範囲を補足してください。"
	case strings.Contains(text, "学生時代"):
		return "student",
			"活動の規模や成果が読み取りづらいです。",
			"人数・期間・結果などの具体的な数値を補足してください。"
	}
	return "generic",
		"この記述は成果や役割の具体性が読み取りづらいです。",
		"成果の数値、役割、工夫点を一文ずつ補足してください。"
}

func selectReviewBlocks(blocks []models.ResumeTextBlock, max int) []models.ResumeTextBlock {
	if len(blocks) == 0 || max <= 0 {
		return nil
	}
	selected := make([]models.ResumeTextBlock, 0, max)
	seenPages := make(map[int]bool)
	for _, block := range blocks {
		text := strings.TrimSpace(block.Text)
		if runeLen(text) < 12 {
			continue
		}
		if strings.HasSuffix(text, "：") || strings.HasSuffix(text, ":") {
			continue
		}
		selected = append(selected, block)
		seenPages[block.PageNumber] = true
		if len(selected) >= max {
			return selected
		}
	}
	if len(selected) < max {
		for _, block := range blocks {
			if seenPages[block.PageNumber] {
				continue
			}
			text := strings.TrimSpace(block.Text)
			if runeLen(text) < 8 {
				continue
			}
			selected = append(selected, block)
			if len(selected) >= max {
				break
			}
		}
	}
	return selected
}

func buildBlockList(blocks []models.ResumeTextBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	var b strings.Builder
	for _, block := range blocks {
		line := strings.TrimSpace(block.Text)
		if line == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("[P%dB%d] %s\n", block.PageNumber, block.BlockIndex, line))
	}
	return b.String()
}

func textMatchScore(block, quote string) int {
	if block == "" || quote == "" {
		return 0
	}
	if strings.Contains(block, quote) || strings.Contains(quote, block) {
		return 100
	}
	blockTokens := splitTokens(block)
	quoteTokens := splitTokens(quote)
	if len(blockTokens) == 0 || len(quoteTokens) == 0 {
		return 0
	}
	score := 0
	for _, token := range quoteTokens {
		for _, b := range blockTokens {
			if token == b {
				score += 10
				break
			}
		}
	}
	return score
}

func splitTokens(s string) []string {
	s = strings.ReplaceAll(s, "。", " ")
	s = strings.ReplaceAll(s, "、", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) < 2 {
			continue
		}
		if !seen[part] {
			seen[part] = true
			result = append(result, part)
		}
	}
	return result
}
