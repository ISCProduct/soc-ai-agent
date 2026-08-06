package services

// newSeekableReader のメモリ上限テスト（Issue #312）
// 実行: cd Backend && go test ./internal/services/... -run TestSeekableReader -v

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/shared"
)

func makeUploadedFileHeader(t *testing.T, filename string, content []byte, contentType string) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	part, err := w.CreatePart(h)
	if err != nil {
		t.Fatalf("CreatePart: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	r := multipart.NewReader(&buf, w.Boundary())
	form, err := r.ReadForm(10 << 20)
	if err != nil {
		t.Fatalf("ReadForm: %v", err)
	}
	files := form.File["file"]
	if len(files) == 0 {
		t.Fatal("file part missing")
	}
	return files[0]
}

type resumeRepoStub struct {
	doc *models.ResumeDocument
}

func (r *resumeRepoStub) CreateDocument(doc *models.ResumeDocument) error {
	if doc.ID == 0 {
		doc.ID = 1
	}
	r.doc = doc
	return nil
}
func (r *resumeRepoStub) UpdateDocument(doc *models.ResumeDocument) error {
	r.doc = doc
	return nil
}
func (r *resumeRepoStub) FindDocumentByID(id uint) (*models.ResumeDocument, error) {
	if r.doc == nil || r.doc.ID != id {
		return nil, errors.New("not found")
	}
	return r.doc, nil
}
func (r *resumeRepoStub) ReplaceTextBlocks(documentID uint, blocks []models.ResumeTextBlock) error {
	return nil
}
func (r *resumeRepoStub) FindTextBlocks(documentID uint) ([]models.ResumeTextBlock, error) {
	return nil, nil
}
func (r *resumeRepoStub) CreateReview(review *models.ResumeReview) error { return nil }
func (r *resumeRepoStub) ReplaceReviewItems(reviewID uint, items []models.ResumeReviewItem) error {
	return nil
}
func (r *resumeRepoStub) FindReviewItems(reviewID uint) ([]models.ResumeReviewItem, error) {
	return nil, nil
}

// TestNewSeekableReader_NormalFile は通常サイズのファイルが正常に読み込まれることを検証する
func TestNewSeekableReader_NormalFile(t *testing.T) {
	content := []byte("Hello, PDF content!")
	rc := io.NopCloser(bytes.NewReader(content))

	sr, err := newSeekableReader(rc)
	if err != nil {
		t.Fatalf("通常ファイルでエラーが発生した: %v", err)
	}
	if sr == nil {
		t.Fatal("seekableReader が nil を返した")
	}

	// 読み込み内容の検証
	got, err := io.ReadAll(sr)
	if err != nil {
		t.Fatalf("読み込みエラー: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("内容が一致しない: got %q, want %q", got, content)
	}
}

// TestNewSeekableReader_OversizedFile は最大サイズを超えるファイルがエラーになることを検証する（#312修正の担保）
func TestNewSeekableReader_OversizedFile(t *testing.T) {
	// maxResumeBytes + 1 バイトのデータを生成
	oversized := bytes.Repeat([]byte("x"), maxResumeBytes+1)
	rc := io.NopCloser(bytes.NewReader(oversized))

	_, err := newSeekableReader(rc)
	if err == nil {
		t.Error("最大サイズを超えるファイルはエラーを返すべき")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("エラーメッセージが不正: got %q", err.Error())
	}
}

// TestNewSeekableReader_ExactlyMaxSize は最大サイズちょうどのファイルが正常に読み込まれることを検証する
func TestNewSeekableReader_ExactlyMaxSize(t *testing.T) {
	exactMax := bytes.Repeat([]byte("x"), maxResumeBytes)
	rc := io.NopCloser(bytes.NewReader(exactMax))

	sr, err := newSeekableReader(rc)
	if err != nil {
		t.Errorf("最大サイズちょうどはエラーになるべきではない: %v", err)
	}
	if sr == nil {
		t.Error("seekableReader が nil を返した")
	}
}

// TestNewSeekableReader_SeekWorks は Seek 操作が正しく動作することを検証する
func TestNewSeekableReader_SeekWorks(t *testing.T) {
	content := []byte("ABCDEFGHIJ")
	rc := io.NopCloser(bytes.NewReader(content))

	sr, err := newSeekableReader(rc)
	if err != nil {
		t.Fatalf("newSeekableReader returned error: %v", err)
	}

	// 5バイト目にシーク
	if _, err := sr.Seek(5, io.SeekStart); err != nil {
		t.Fatalf("Seek returned error: %v", err)
	}

	buf := make([]byte, 3)
	if _, err := sr.Read(buf); err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if string(buf) != "FGH" {
		t.Errorf("Seek後の読み込み結果が不正: got %q, want %q", buf, "FGH")
	}
}

func TestValidateFileUpload_AllowsOctetStreamWithPDFMagic(t *testing.T) {
	fh := makeUploadedFileHeader(t, "resume.pdf", []byte("%PDF-1.4\n%mock"), "application/octet-stream")
	if err := validateFileUpload(fh); err != nil {
		t.Fatalf("PDF magic + octet-stream は許可すべき: %v", err)
	}
}

func TestValidateFileUpload_RejectsNonDocument(t *testing.T) {
	fh := makeUploadedFileHeader(t, "notes.txt", []byte("hello world text"), "text/plain")
	err := validateFileUpload(fh)
	var ve *shared.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("ValidationError を期待: got %v", err)
	}
}

func TestResumeService_UploadStoresLocallyWithoutS3(t *testing.T) {
	dir := t.TempDir()
	repo := &resumeRepoStub{}
	svc := &ResumeService{repo: repo, storageDir: dir}
	fh := makeUploadedFileHeader(t, "resume.pdf", []byte("%PDF-1.4\n%mock-content"), "application/octet-stream")

	result, err := svc.Upload(7, "sess-1", "pdf", "", fh)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
	if result.Document == nil || result.Document.ID == 0 {
		t.Fatalf("document missing: %+v", result)
	}
	stored := result.Document.StoredPath
	if !strings.HasPrefix(stored, dir) {
		t.Fatalf("local path expected under %s, got %s", dir, stored)
	}
	if _, err := os.Stat(stored); err != nil {
		t.Fatalf("stored file missing: %v", err)
	}
	wantSuffix := filepath.Join("7", "1", "original.pdf")
	if !strings.HasSuffix(stored, wantSuffix) {
		t.Fatalf("unexpected path layout: got %s want suffix %s", stored, wantSuffix)
	}
}

func TestResumeService_OpenAnnotatedFileRejectsOtherUser(t *testing.T) {
	service := NewResumeService(&resumeRepoStub{
		doc: &models.ResumeDocument{
			ID:            10,
			UserID:        1,
			AnnotatedPath: "/tmp/annotated.pdf",
		},
	}, "", nil)

	_, err := service.OpenAnnotatedFile(10, 2)
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("他ユーザーの注釈PDF取得は forbidden であるべき: got %v", err)
	}
}

func TestResumeService_ReviewDocumentRejectsOtherUser(t *testing.T) {
	service := NewResumeService(&resumeRepoStub{
		doc: &models.ResumeDocument{
			ID:     10,
			UserID: 1,
		},
	}, "", nil)

	_, _, err := service.ReviewDocument(10, 2, "ACME", "Engineer", "new_grad")
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("他ユーザーのレビュー実行は forbidden であるべき: got %v", err)
	}
}

func TestResumeService_ReviewDocumentStreamRejectsOtherUser(t *testing.T) {
	service := NewResumeService(&resumeRepoStub{
		doc: &models.ResumeDocument{
			ID:     10,
			UserID: 1,
		},
	}, "", nil)
	rr := httptest.NewRecorder()

	err := service.ReviewDocumentStream(context.Background(), 10, 2, "ACME", "Engineer", "new_grad", rr)
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("他ユーザーのストリーミングレビューは forbidden であるべき: got %v", err)
	}
	if !strings.Contains(rr.Body.String(), "forbidden") {
		t.Fatalf("SSE エラーに forbidden が含まれるべき: got %q", rr.Body.String())
	}
}
