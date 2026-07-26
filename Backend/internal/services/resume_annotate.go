package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"Backend/internal/models"
)

func (s *ResumeService) OpenAnnotatedFile(documentID uint, requestingUserID uint) (*AnnotatedFile, error) {
	doc, err := s.repo.FindDocumentByID(documentID)
	if err != nil {
		return nil, err
	}
	if doc.UserID != requestingUserID {
		return nil, ErrForbidden
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
	reader, err := newSeekableReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to load resume file: %w", err)
	}
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
			bbox, _ := json.Marshal(map[string]any{
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
	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		bboxInfo := decodeBBoxInfo(item.BBox)
		payload = append(payload, map[string]any{
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
		log.Printf("annotate script error: %v\nstderr:\n%s", err, stderr.String())
		return "", "", errors.New("document processing failed")
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
