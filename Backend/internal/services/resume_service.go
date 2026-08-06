package services

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services/flywheel"
	"Backend/internal/services/shared"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/url"
	"path/filepath"
	"strings"
)

// ValidationError はユーザー入力起因のエラーを表す。controller で 422 を返すために使用する。
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// allowedMIMETypes はアップロード可能なファイルタイプ
var allowedMIMETypes = map[string]bool{
	"application/pdf":    true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
}

// pdfMagicBytes はPDFファイルの先頭シグネチャ（%PDF）
var pdfMagicBytes = []byte{0x25, 0x50, 0x44, 0x46}

// validateFileUpload はMIMEタイプとファイルシグネチャ（magic bytes）を検証する。
// Content-Type が空や application/octet-stream でも、magic bytes が一致すれば許可する。
func validateFileUpload(fileHeader *multipart.FileHeader) error {
	f, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 4)
	if _, err := io.ReadFull(f, buf); err != nil {
		return &ValidationError{Message: "ファイルが小さすぎるか破損しています"}
	}

	magicOK := bytes.HasPrefix(buf, pdfMagicBytes) ||
		bytes.HasPrefix(buf, []byte{0x50, 0x4B, 0x03, 0x04}) || // DOCX (ZIP)
		bytes.HasPrefix(buf, []byte{0xD0, 0xCF, 0x11, 0xE0}) // DOC (OLE2)
	if magicOK {
		return nil
	}

	mimeType := fileHeader.Header.Get("Content-Type")
	if allowedMIMETypes[mimeType] {
		return &ValidationError{Message: "ファイル内容が PDF / Word 形式と一致しません"}
	}
	return &ValidationError{Message: "PDF または Word（.doc/.docx）のみアップロードできます"}
}

// validateURL はSSRF対策のためURLスキームとIPアドレス範囲を検証する
func validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return &ValidationError{Message: "only http/https URLs are allowed"}
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
		return &ValidationError{Message: "requests to internal IP addresses are not allowed"}
	}
	return nil
}

// validatePathInDir はpathがdir配下であることを確認する（パストラバーサル対策）
func validatePathInDir(path, dir string) error {
	cleanPath := filepath.Clean(path)
	cleanDir := filepath.Clean(dir)
	if !strings.HasPrefix(cleanPath, cleanDir+string(filepath.Separator)) && cleanPath != cleanDir {
		return fmt.Errorf("path %q is outside allowed directory", path)
	}
	return nil
}

type ResumeService struct {
	repo         repository.ResumeRepository
	storageDir   string
	aiClient     *openai.Client
	s3           *s3Storage
	s3Err        error
	crossFeature *flywheel.CrossFeatureIntegrationService
	validator    *CompanyValidationService
	companyRepo  shared.CompanyBriefReader
}

// SetCrossFeatureService 機能間連携サービスを注入する（オプション）
func (s *ResumeService) SetCrossFeatureService(cf *flywheel.CrossFeatureIntegrationService) {
	s.crossFeature = cf
}

// SetCompanyValidator 企業実在確認サービスを注入する（オプション）
func (s *ResumeService) SetCompanyValidator(v *CompanyValidationService) {
	s.validator = v
}

// SetCompanyRepo 企業共有キャッシュ参照用リポジトリを注入する（オプション）
func (s *ResumeService) SetCompanyRepo(r shared.CompanyBriefReader) {
	s.companyRepo = r
}

func (s *ResumeService) lookupCompanyBriefFromCache(companyName string) string {
	if s.companyRepo == nil {
		return ""
	}
	name := strings.TrimSpace(companyName)
	if name == "" {
		return ""
	}
	company, err := s.companyRepo.FindByName(name)
	if err != nil || company == nil {
		return ""
	}
	var profile *models.CompanyWeightProfile
	if p, err := s.companyRepo.GetWeightProfile(company.ID, nil); err == nil {
		profile = p
	}
	return BuildCompanyBrief(company, profile)
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

func (s *ResumeService) EnsureDocumentOwner(documentID uint, requestingUserID uint) error {
	doc, err := s.repo.FindDocumentByID(documentID)
	if err != nil {
		return err
	}
	if doc.UserID != requestingUserID {
		return shared.ErrForbidden
	}
	return nil
}

// ensureRealCompany は企業名が指定されている場合に実在確認を行い、正規化名を返す。
// 企業名が空の場合（職種のみレビュー）はそのまま通す。
func (s *ResumeService) ensureRealCompany(ctx context.Context, companyName string) (string, error) {
	name := strings.TrimSpace(companyName)
	if name == "" {
		return "", nil
	}
	if s.validator == nil {
		return "", &ValidationError{Message: "企業の実在確認機能が利用できません。しばらくしてから再度お試しください"}
	}
	result, err := s.validator.Validate(ctx, name)
	if err != nil {
		return "", err
	}
	if !result.Exists {
		return "", &ValidationError{Message: "実在が確認できない企業名です。企業を検索して候補から選択してください"}
	}
	if strings.TrimSpace(result.CanonicalName) != "" {
		return result.CanonicalName, nil
	}
	return name, nil
}

type AnnotatedFile struct {
	Reader      io.ReadSeeker
	Size        int64
	ContentType string
	Filename    string
	CloseFunc   func() error
}
