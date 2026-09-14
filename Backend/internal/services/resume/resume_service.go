package resume

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services/company"
	"Backend/internal/services/flywheel"
	"Backend/internal/services/shared"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

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
		return &shared.ValidationError{Message: "ファイルが小さすぎるか破損しています"}
	}

	magicOK := bytes.HasPrefix(buf, pdfMagicBytes) ||
		bytes.HasPrefix(buf, []byte{0x50, 0x4B, 0x03, 0x04}) || // DOCX (ZIP)
		bytes.HasPrefix(buf, []byte{0xD0, 0xCF, 0x11, 0xE0}) // DOC (OLE2)
	if magicOK {
		return nil
	}

	mimeType := fileHeader.Header.Get("Content-Type")
	if allowedMIMETypes[mimeType] {
		return &shared.ValidationError{Message: "ファイル内容が PDF / Word 形式と一致しません"}
	}
	return &shared.ValidationError{Message: "PDF または Word（.doc/.docx）のみアップロードできます"}
}

// lookupIP はホスト名からIPアドレスを解決する。テストでモック可能にするため変数にしている。
var lookupIP = net.LookupIP

// isInternalIP はループバック/プライベート/リンクローカル/未指定アドレスのいずれかを判定する（SSRF対策）
func isInternalIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

// validateURL はSSRF対策のためURLスキームとIPアドレス範囲を検証する。
// ホスト名がIPリテラルでない場合は名前解決を行い、解決された全IPを検証する
// （ホスト名だけを見て素通りさせないことで内部IPへのDNS解決を防ぐ）。
func validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return &shared.ValidationError{Message: "only http/https URLs are allowed"}
	}
	host := parsed.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if isInternalIP(ip) {
			return &shared.ValidationError{Message: "requests to internal IP addresses are not allowed"}
		}
		return nil
	}
	ips, err := lookupIP(host)
	if err != nil {
		return &shared.ValidationError{Message: "failed to resolve host"}
	}
	for _, ip := range ips {
		if isInternalIP(ip) {
			return &shared.ValidationError{Message: "requests to internal IP addresses are not allowed"}
		}
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
	validator    *company.CompanyValidationService
	companyRepo  shared.CompanyBriefReader
	persona      PersonaEnsurer
	provisioner  CompanyProvisioner
}

// CompanyProvisioner は DB に無い企業を取得して登録する。
//
// 実装は CompanyInfoFetcher.ProvisionByName。gBizinfo（無料）を優先し、
// 足りなければ AI 検索へ落ちる。取得結果は companies に保存されるため、
// 以後その企業は履歴書レビュー・企業検索・マッチング・面接ヒントのすべてから
// 追加コストなしで参照できる。
type CompanyProvisioner interface {
	ProvisionByName(ctx context.Context, companyName string) (*models.Company, error)
}

// SetCompanyProvisioner は未登録企業の取得・登録器を注入する（オプション）。
func (s *ResumeService) SetCompanyProvisioner(p CompanyProvisioner) {
	s.provisioner = p
}

// PersonaEnsurer は企業の「求める人材像」(CompanyWeightProfile) を用意する。
//
// 実装は JobFetchService.FetchAndSavePersona で、DB に保存済みの企業情報だけを
// 材料に安いモデルで1回生成し、結果を company_weight_profiles に保存する
// （Web検索はしない）。2回目以降は AI を呼ばずに DB から返る。
type PersonaEnsurer interface {
	FetchAndSavePersona(ctx context.Context, companyID uint, forceRefresh bool) (*models.CompanyWeightProfile, error)
}

// SetPersonaEnsurer は求める人材像の生成器を注入する（オプション）。
func (s *ResumeService) SetPersonaEnsurer(p PersonaEnsurer) {
	s.persona = p
}

// SetCrossFeatureService 機能間連携サービスを注入する（オプション）
func (s *ResumeService) SetCrossFeatureService(cf *flywheel.CrossFeatureIntegrationService) {
	s.crossFeature = cf
}

// SetCompanyValidator 企業実在確認サービスを注入する（オプション）
func (s *ResumeService) SetCompanyValidator(v *company.CompanyValidationService) {
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
	comp, err := s.companyRepo.FindByName(name)
	if err != nil || comp == nil {
		return ""
	}
	var profile *models.CompanyWeightProfile
	if p, err := s.companyRepo.GetWeightProfile(comp.ID, nil); err == nil {
		profile = p
	}
	// 未生成なら1回だけ作る（#1124）。
	//
	// brief の「重視傾向」が企業の求める人材像そのもので、これが無いと
	// レビューは業種・事業内容だけを見た一般論になる。実測では 842社中
	// 90社(10.7%)しか持っていなかった。
	//
	// 生成は DB の企業情報だけを材料にした安いモデルの1コールで、結果は
	// DB に保存され全学生のレビューで使い回される。失敗してもレビューは
	// 続行する（重視傾向が無い状態に戻るだけ）。
	if profile == nil && s.persona != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if p, err := s.persona.FetchAndSavePersona(ctx, comp.ID, false); err == nil {
			profile = p
		} else {
			log.Printf("[Resume] persona ensure failed company_id=%d: %v", comp.ID, err)
		}
	}
	return company.BuildCompanyBrief(comp, profile)
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
	if _, err := s.repo.FindDocumentByIDForUser(documentID, requestingUserID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return shared.ErrForbidden
		}
		return err
	}
	return nil
}

// ensureRealCompany は企業名が指定されている場合に実在確認を行い、正規化名を返す。
// 企業名が空の場合（職種のみレビュー）はそのまま通す。
func (s *ResumeService) ensureRealCompany(companyName string) (string, error) {
	name := strings.TrimSpace(companyName)
	if name == "" {
		return "", nil
	}
	if s.validator == nil {
		return "", &shared.ValidationError{Message: "企業の実在確認機能が利用できません。しばらくしてから再度お試しください"}
	}
	// まず DB だけで確定させる（#1124）。
	//
	// 従来は DB に無いと Web検索で実在確認だけを行い、**結果を捨てていた**。
	// 1コールあたり約3万入力トークン払って真偽値しか得られず、企業情報が無いので
	// レビューは一般論になり、次に同じ企業名が来ればまた払っていた。
	result, err := s.validator.ValidateFromDB(name)
	if err != nil {
		return "", err
	}
	if result != nil && result.Exists {
		if strings.TrimSpace(result.CanonicalName) != "" {
			return result.CanonicalName, nil
		}
		return name, nil
	}

	// DB に無ければ取得して登録する。取得結果は companies に保存されるので、
	// 以後この企業は企業検索・マッチング・面接ヒントからも参照できる。
	if s.provisioner == nil {
		return "", &shared.ValidationError{Message: "登録されていない企業名です。企業を検索して候補から選択してください"}
	}
	provisionCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	company, err := s.provisioner.ProvisionByName(provisionCtx, name)
	if err != nil {
		log.Printf("[Resume] company provision failed name=%q: %v", name, err)
		return "", &shared.ValidationError{Message: "企業情報を取得できませんでした。企業名を確認するか、企業を検索して候補から選択してください"}
	}
	return company.Name, nil
}

type AnnotatedFile struct {
	Reader      io.ReadSeeker
	Size        int64
	ContentType string
	Filename    string
	CloseFunc   func() error
}
