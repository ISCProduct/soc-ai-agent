package resume

import (
	"Backend/internal/services/shared"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"Backend/internal/models"
)

func (s *ResumeService) Upload(userID uint, sessionID, sourceType, sourceURL string, fileHeader *multipart.FileHeader) (*ResumeUploadResult, error) {
	if strings.TrimSpace(sourceType) == "" {
		sourceType = "pdf"
	}
	if fileHeader == nil && strings.TrimSpace(sourceURL) == "" {
		return nil, &shared.ValidationError{Message: "ファイルまたは source_url が必要です"}
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
		if err := validateFileUpload(fileHeader); err != nil {
			return nil, err
		}
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
		if err := validateURL(sourceURL); err != nil {
			return nil, err
		}
		downloaded, filename, err := downloadSourceFile(context.Background(), sourceURL, workDir)
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
		storedPath, err := s.persistUploadedFile(doc, doc.StoredPath, filename)
		if err != nil {
			return nil, err
		}
		doc.StoredPath = storedPath
		if err := s.repo.UpdateDocument(doc); err != nil {
			return nil, fmt.Errorf("failed to update document: %w", err)
		}
	}

	return &ResumeUploadResult{Document: doc}, nil
}

// persistUploadedFile は S3 が使える場合は S3 へ、不可ならローカル storageDir へ永続化する。
func (s *ResumeService) persistUploadedFile(doc *models.ResumeDocument, localPath, filename string) (string, error) {
	if s.s3 != nil && s.s3.isEnabled() {
		if err := s.ensureS3Available(); err != nil {
			return "", &shared.ValidationError{Message: "ファイルストレージの準備に失敗しました。しばらくしてから再度お試しください"}
		}
		return s.uploadToS3(context.Background(), doc, localPath, filename)
	}

	dir := filepath.Join(s.storageDir, fmt.Sprintf("%d", doc.UserID), fmt.Sprintf("%d", doc.ID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create local storage dir: %w", err)
	}
	dest := filepath.Join(dir, filename)
	if err := copyFile(localPath, dest); err != nil {
		return "", fmt.Errorf("failed to store file locally: %w", err)
	}
	log.Printf("resume_upload: stored locally path=%s (S3 unavailable)", dest)
	return dest, nil
}

type seekableReader struct {
	data []byte
	r    *bytes.Reader
}

// maxResumeBytes はメモリ展開を許可する最大ファイルサイズ（100 MB）
const maxResumeBytes = 100 * 1024 * 1024

// newSeekableReader は src を全量メモリに読み込んで Seek 可能なリーダーを返す。
// PDF解析ライブラリが io.ReadSeeker を要求するため全量読み込みが必要だが、
// OOM を防ぐため maxResumeBytes を超えるファイルはエラーを返す。
func newSeekableReader(src io.ReadCloser) (*seekableReader, error) {
	defer src.Close()
	lr := io.LimitReader(src, maxResumeBytes+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	if int64(len(data)) > maxResumeBytes {
		return nil, fmt.Errorf("file size exceeds maximum allowed size (%d MB)", maxResumeBytes/1024/1024)
	}
	return &seekableReader{
		data: data,
		r:    bytes.NewReader(data),
	}, nil
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

// MaxResumeBytes is the exported constant for use in external tests.
const MaxResumeBytes = maxResumeBytes

// NewSeekableReader is an exported wrapper for newSeekableReader for use in external tests.
func NewSeekableReader(src io.ReadCloser) (io.ReadSeekCloser, error) {
	return newSeekableReader(src)
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

// ssrfSafeDialContext は接続直前に実際に使うIPを再検証するDialContext。
// validateURLでの検証後にDNSレコードが書き換わる（DNSリバインディング）TOCTOUを防ぐため、
// ここで解決したIPをそのまま宛先に使い、標準ダイヤラに再度ホスト名解決させない。
func ssrfSafeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if ip := net.ParseIP(host); ip != nil {
		if isInternalIP(ip) {
			return nil, fmt.Errorf("blocked request to internal address: %s", host)
		}
		return dialer.DialContext(ctx, network, addr)
	}
	ips, err := lookupIP(host)
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("failed to resolve host: %s", host)
	}
	for _, ip := range ips {
		if isInternalIP(ip) {
			return nil, fmt.Errorf("blocked request to internal address: %s", host)
		}
	}
	var lastErr error
	for _, ip := range ips {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("failed to resolve host: %s", host)
	}
	return nil, lastErr
}

func downloadSourceFile(ctx context.Context, url, storagePath string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{DialContext: ssrfSafeDialContext},
	}
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
