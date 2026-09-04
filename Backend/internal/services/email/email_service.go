package email

import (
	"Backend/domain/entity"
	"Backend/internal/services/analysis"
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/resend/resend-go/v3"
)

// EmailService はトランザクションメール送信を担うサービス（#756）。
// EMAIL_PROVIDER=resend|smtp|log でバックエンドを切替える。未設定時は
// RESEND_API_KEY → SMTP_HOST → log の順で自動選択する。
type EmailService struct {
	provider  string
	from      string
	transport mailTransport
}

// NewEmailService は環境変数から送信プロバイダを解決して EmailService を生成する。
func NewEmailService() *EmailService {
	from := firstNonEmptyEnv(
		os.Getenv("EMAIL_FROM"),
		os.Getenv("RESEND_FROM"),
		os.Getenv("SMTP_FROM"),
		"noreply@example.com",
	)
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("EMAIL_PROVIDER")))
	if provider == "" {
		switch {
		case strings.TrimSpace(os.Getenv("RESEND_API_KEY")) != "":
			provider = "resend"
		case strings.TrimSpace(os.Getenv("SMTP_HOST")) != "":
			provider = "smtp"
		default:
			provider = "log"
		}
	}

	svc := &EmailService{provider: provider, from: from}
	switch provider {
	case "resend":
		apiKey := strings.TrimSpace(os.Getenv("RESEND_API_KEY"))
		if apiKey == "" {
			log.Printf("[EmailService] EMAIL_PROVIDER=resend だが RESEND_API_KEY 未設定のため log にフォールバックします")
			svc.provider = "log"
			svc.transport = &logMailTransport{}
			break
		}
		svc.transport = &resendMailTransport{client: resend.NewClient(apiKey)}
	case "smtp":
		port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
		if port == 0 {
			port = 587
		}
		host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
		if host == "" {
			log.Printf("[EmailService] EMAIL_PROVIDER=smtp だが SMTP_HOST 未設定のため log にフォールバックします")
			svc.provider = "log"
			svc.transport = &logMailTransport{}
			break
		}
		svc.transport = &smtpMailTransport{
			host:     host,
			port:     port,
			user:     os.Getenv("SMTP_USER"),
			password: os.Getenv("SMTP_PASSWORD"),
		}
	default:
		svc.provider = "log"
		svc.transport = &logMailTransport{}
	}
	log.Printf("[EmailService] provider=%s from=%s\n", svc.provider, svc.from)
	return svc
}

func firstNonEmptyEnv(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// Provider は現在の送信バックエンド名を返す（テスト・診断用）。
func (s *EmailService) Provider() string {
	if s == nil || s.provider == "" {
		return "log"
	}
	return s.provider
}

func (s *EmailService) sendHTML(to []string, subject, htmlBody string) error {
	return s.dispatch(mailMessage{From: s.from, To: to, Subject: subject, HTML: htmlBody})
}

func (s *EmailService) sendText(to []string, subject, textBody string) error {
	return s.dispatch(mailMessage{From: s.from, To: to, Subject: subject, Text: textBody})
}

func (s *EmailService) dispatch(msg mailMessage) error {
	if s == nil {
		return fmt.Errorf("email service is nil")
	}
	if s.transport == nil {
		s.transport = &logMailTransport{}
		s.provider = "log"
	}
	if msg.From == "" {
		msg.From = s.from
	}
	if len(msg.To) == 0 {
		return nil
	}
	return s.transport.Send(msg)
}

// EmailReportCompany は分析レポートメールに含めるおすすめ企業1件分のデータ。
type EmailReportCompany struct {
	Rank   int
	Name   string
	Score  int
	Reason string
}

// emailReportData は reportEmailTemplate に渡す内部用テンプレートデータ。
// スコア・進捗・AIコメント・おすすめ企業を保持する。
type emailReportData struct {
	UserName              string
	SessionID             string
	SentAt                string
	JobScore              string
	InterestScore         string
	AptitudeScore         string
	FutureScore           string
	JobProgress           string
	InterestProgress      string
	AptitudeProgress      string
	FutureProgress        string
	Companies             []EmailReportCompany
	ScoreComment          string
	JobSuitabilityComment string
}

// reportEmailTemplate はAI就活分析レポートメール用のHTMLテンプレート。
// emailReportData を渡して Execute する。
const reportEmailTemplate = `<!DOCTYPE html>
<html lang="ja">
<head>
  <meta charset="UTF-8">
  <title>AI就活分析レポート</title>
  <style>
    body{font-family:'Hiragino Sans','Meiryo',sans-serif;background:#f5f5f5;margin:0;padding:20px;}
    .container{max-width:600px;margin:0 auto;background:#fff;border-radius:8px;overflow:hidden;box-shadow:0 2px 8px rgba(0,0,0,0.1);}
    .header{background:linear-gradient(135deg,#1976D2,#42A5F5);color:white;padding:32px 24px;text-align:center;}
    .header h1{margin:0;font-size:22px;}
    .header p{margin:8px 0 0;opacity:.9;font-size:13px;}
    .section{padding:20px 24px;border-bottom:1px solid #e0e0e0;}
    .section h2{margin:0 0 14px;font-size:16px;color:#1976D2;}
    .scores-grid{display:grid;grid-template-columns:1fr 1fr;gap:10px;}
    .score-card{background:#f5f5f5;border-radius:8px;padding:12px;text-align:center;}
    .score-label{font-size:12px;color:#666;margin-bottom:4px;}
    .score-value{font-size:26px;font-weight:bold;color:#1976D2;}
    .phase-item{margin-bottom:10px;}
    .phase-header{display:flex;justify-content:space-between;font-size:13px;margin-bottom:4px;}
    .bar-bg{background:#e0e0e0;border-radius:4px;height:8px;}
    .bar-fill{background:#1976D2;border-radius:4px;height:8px;}
    .company-item{background:#f9f9f9;border-radius:8px;padding:14px;margin-bottom:10px;border-left:4px solid #1976D2;}
    .company-rank{font-size:11px;color:#888;}
    .company-name{font-size:15px;font-weight:bold;margin:3px 0;}
    .company-score{font-size:18px;font-weight:bold;color:#1976D2;}
    .company-reason{font-size:12px;color:#555;margin-top:6px;line-height:1.5;}
    .footer{padding:20px 24px;text-align:center;background:#fafafa;color:#999;font-size:11px;}
  </style>
</head>
<body>
<div class="container">
  <div class="header">
    <h1>🎉 AI就活分析レポート</h1>
    <p>{{.UserName}} さんの分析結果</p>
    <p>{{.SentAt}}</p>
  </div>

  <div class="section">
    <h2>📊 4段階分析スコア</h2>
    <div class="scores-grid">
      <div class="score-card">
        <div class="score-label">職種分析</div>
        <div class="score-value">{{.JobScore}}%</div>
      </div>
      <div class="score-card">
        <div class="score-label">興味分析</div>
        <div class="score-value">{{.InterestScore}}%</div>
      </div>
      <div class="score-card">
        <div class="score-label">適性分析</div>
        <div class="score-value">{{.AptitudeScore}}%</div>
      </div>
      <div class="score-card">
        <div class="score-label">将来分析</div>
        <div class="score-value">{{.FutureScore}}%</div>
      </div>
    </div>
  </div>

  {{if .ScoreComment}}
  <div class="section">
    <h2>💬 分析コメント</h2>
    <p style="font-size:13px;line-height:1.8;color:#333;margin:0;">{{.ScoreComment}}</p>
  </div>
  {{end}}

  {{if .JobSuitabilityComment}}
  <div class="section">
    <h2>💼 職種適性コメント</h2>
    <p style="font-size:13px;line-height:1.8;color:#333;margin:0;">{{.JobSuitabilityComment}}</p>
  </div>
  {{end}}

  <div class="section">
    <h2>📈 フェーズ進捗</h2>
    <div class="phase-item">
      <div class="phase-header"><span>職種分析</span><span>{{.JobProgress}}%</span></div>
      <div class="bar-bg"><div class="bar-fill" style="width:{{.JobProgress}}%"></div></div>
    </div>
    <div class="phase-item">
      <div class="phase-header"><span>興味分析</span><span>{{.InterestProgress}}%</span></div>
      <div class="bar-bg"><div class="bar-fill" style="width:{{.InterestProgress}}%"></div></div>
    </div>
    <div class="phase-item">
      <div class="phase-header"><span>適性分析</span><span>{{.AptitudeProgress}}%</span></div>
      <div class="bar-bg"><div class="bar-fill" style="width:{{.AptitudeProgress}}%"></div></div>
    </div>
    <div class="phase-item">
      <div class="phase-header"><span>将来分析</span><span>{{.FutureProgress}}%</span></div>
      <div class="bar-bg"><div class="bar-fill" style="width:{{.FutureProgress}}%"></div></div>
    </div>
  </div>

  {{if .Companies}}
  <div class="section">
    <h2>🏢 おすすめ企業</h2>
    {{range .Companies}}
    <div class="company-item">
      <div class="company-rank">第{{.Rank}}位</div>
      <div class="company-name">{{.Name}}</div>
      <div class="company-score">適合度 {{.Score}}%</div>
      <div class="company-reason">{{.Reason}}</div>
    </div>
    {{end}}
  </div>
  {{end}}

  <div class="footer">
    <p>このメールはAI就活エージェントから自動送信されました。</p>
    <p>セッションID: {{.SessionID}}</p>
  </div>
</div>
</body>
</html>`

// SendAnalysisReport はAI就活分析レポートをユーザーのメールアドレスへ送信する。
// summary にはスコア・進捗・AIコメント、companies にはおすすめ企業リストを渡す。
// SMTP未設定時はログ出力のみでエラーを返さない（開発環境フォールバック）。
func (s *EmailService) SendAnalysisReport(user *entity.User, summary *analysis.AnalysisSummary, companies []EmailReportCompany, sessionID string) error {
	tmpl, err := template.New("report").Parse(reportEmailTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse email template: %w", err)
	}

	// pct は 0〜1 のスコアを 0〜100% の整数文字列に変換するヘルパー
	pct := func(v float64) string { return fmt.Sprintf("%.0f", v*100) }
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)

	data := emailReportData{
		UserName:              user.Name,
		SessionID:             sessionID,
		SentAt:                time.Now().In(jst).Format("2006年01月02日 15:04"),
		JobScore:              fmt.Sprintf("%.0f", summary.Scores.JobScore*100),
		InterestScore:         fmt.Sprintf("%.0f", summary.Scores.InterestScore*100),
		AptitudeScore:         fmt.Sprintf("%.0f", summary.Scores.AptitudeScore*100),
		FutureScore:           fmt.Sprintf("%.0f", summary.Scores.FutureScore*100),
		JobProgress:           pct(summary.Progress.Job),
		InterestProgress:      pct(summary.Progress.Interest),
		AptitudeProgress:      pct(summary.Progress.Aptitude),
		FutureProgress:        pct(summary.Progress.Future),
		Companies:             companies,
		ScoreComment:          summary.ScoreComment,
		JobSuitabilityComment: summary.JobSuitabilityComment,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}

	htmlBody := buf.String()
	return s.sendHTML([]string{user.Email}, "AI就活分析レポート", htmlBody)
}

// SendVerificationEmail メール認証用のメールを送信
func (s *EmailService) SendVerificationEmail(user *entity.User, token, appURL string) error {
	verifyURL := appURL + "/verify-email?token=" + token
	safeName := template.HTMLEscapeString(user.Name)
	body := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ja"><head><meta charset="UTF-8"><title>メール認証</title></head>
<body style="font-family:sans-serif;background:#f5f5f5;padding:20px;">
<div style="max-width:500px;margin:0 auto;background:#fff;border-radius:8px;padding:32px;">
<h2 style="color:#1976D2;">メールアドレスの確認</h2>
<p>%s さん、ご登録ありがとうございます。</p>
<p>以下のボタンをクリックしてメールアドレスを確認してください。</p>
<a href="%s" style="display:inline-block;background:#1976D2;color:#fff;padding:12px 24px;border-radius:6px;text-decoration:none;font-weight:bold;margin:16px 0;">メールアドレスを確認する</a>
<p style="color:#888;font-size:12px;">このリンクは24時間有効です。身に覚えのない場合は無視してください。</p>
</div>
</body></html>`, safeName, verifyURL)

	return s.sendHTML([]string{user.Email}, "メールアドレスの確認", body)
}

// SendReVerificationEmail 再認証メールを送信
func (s *EmailService) SendReVerificationEmail(user *entity.User, token, appURL string) error {
	verifyURL := appURL + "/verify-email?token=" + token
	safeName := template.HTMLEscapeString(user.Name)
	body := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ja"><head><meta charset="UTF-8"><title>再認証</title></head>
<body style="font-family:sans-serif;background:#f5f5f5;padding:20px;">
<div style="max-width:500px;margin:0 auto;background:#fff;border-radius:8px;padding:32px;">
<h2 style="color:#ec5b13;">ログイン再認証のお願い</h2>
<p>%s さん、前回のログインから10日以上が経過しました。</p>
<p>セキュリティのため、以下のボタンをクリックして再認証を行ってください。</p>
<a href="%s" style="display:inline-block;background:#ec5b13;color:#fff;padding:12px 24px;border-radius:6px;text-decoration:none;font-weight:bold;margin:16px 0;">再認証する</a>
<p style="color:#888;font-size:12px;">このリンクは24時間有効です。</p>
</div>
</body></html>`, safeName, verifyURL)

	return s.sendHTML([]string{user.Email}, "ログイン再認証のお願い", body)
}

// InterviewReportEmailData 面接レポートメールのデータ
type InterviewReportEmailData struct {
	UserName   string
	SessionID  string
	SentAt     string
	Summary    []string
	LogicScore int
	SpecScore  int
	OwnScore   int
	LogicEvid  string
	SpecEvid   string
	OwnEvid    string
}

// interviewReportEmailTemplate はAI面接練習レポートメール用のHTMLテンプレート。
// InterviewReportEmailData を渡して Execute する。
const interviewReportEmailTemplate = `<!DOCTYPE html>
<html lang="ja">
<head>
  <meta charset="UTF-8">
  <title>AI面接練習レポート</title>
  <style>
    body{font-family:'Hiragino Sans','Meiryo',sans-serif;background:#f5f5f5;margin:0;padding:20px;}
    .container{max-width:600px;margin:0 auto;background:#fff;border-radius:8px;overflow:hidden;box-shadow:0 2px 8px rgba(0,0,0,0.1);}
    .header{background:linear-gradient(135deg,#ec5b13,#ff8a50);color:white;padding:32px 24px;text-align:center;}
    .header h1{margin:0;font-size:22px;}
    .header p{margin:8px 0 0;opacity:.9;font-size:13px;}
    .section{padding:20px 24px;border-bottom:1px solid #e0e0e0;}
    .section h2{margin:0 0 14px;font-size:16px;color:#ec5b13;}
    .summary-item{display:flex;gap:8px;margin-bottom:8px;font-size:13px;line-height:1.6;color:#333;}
    .bullet{color:#ec5b13;font-weight:bold;flex-shrink:0;}
    .scores-grid{display:grid;grid-template-columns:1fr 1fr 1fr;gap:10px;}
    .score-card{background:#fff8f5;border:1px solid #ffd5c0;border-radius:8px;padding:12px;text-align:center;}
    .score-label{font-size:11px;color:#888;margin-bottom:4px;}
    .score-value{font-size:28px;font-weight:bold;color:#ec5b13;}
    .score-max{font-size:12px;color:#aaa;}
    .evidence-item{background:#f9f9f9;border-radius:6px;padding:12px;margin-bottom:8px;}
    .evidence-label{font-size:11px;color:#ec5b13;font-weight:bold;margin-bottom:4px;}
    .evidence-text{font-size:13px;color:#555;line-height:1.5;}
    .footer{padding:20px 24px;text-align:center;background:#fafafa;color:#999;font-size:11px;}
  </style>
</head>
<body>
<div class="container">
  <div class="header">
    <h1>AI面接練習レポート</h1>
    <p>{{.UserName}} さんの面接結果</p>
    <p>{{.SentAt}}</p>
  </div>
  {{if .Summary}}
  <div class="section">
    <h2>総合フィードバック</h2>
    {{range .Summary}}<div class="summary-item"><span class="bullet">▶</span><span>{{.}}</span></div>{{end}}
  </div>
  {{end}}
  <div class="section">
    <h2>評価スコア（5点満点）</h2>
    <div class="scores-grid">
      <div class="score-card"><div class="score-label">論理性</div><div class="score-value">{{.LogicScore}}</div><div class="score-max">/ 5</div></div>
      <div class="score-card"><div class="score-label">具体性</div><div class="score-value">{{.SpecScore}}</div><div class="score-max">/ 5</div></div>
      <div class="score-card"><div class="score-label">主体性</div><div class="score-value">{{.OwnScore}}</div><div class="score-max">/ 5</div></div>
    </div>
  </div>
  <div class="section">
    <h2>評価の根拠</h2>
    {{if .LogicEvid}}<div class="evidence-item"><div class="evidence-label">論理性</div><div class="evidence-text">{{.LogicEvid}}</div></div>{{end}}
    {{if .SpecEvid}}<div class="evidence-item"><div class="evidence-label">具体性</div><div class="evidence-text">{{.SpecEvid}}</div></div>{{end}}
    {{if .OwnEvid}}<div class="evidence-item"><div class="evidence-label">主体性</div><div class="evidence-text">{{.OwnEvid}}</div></div>{{end}}
  </div>
  <div class="footer"><p>このメールはAI就活エージェントから自動送信されました。</p><p>セッションID: {{.SessionID}}</p></div>
</div>
</body>
</html>`

// SendRegistrationEmail 仮登録確認メールを送信（メールアドレスのみ、Userオブジェクト不要）
func (s *EmailService) SendRegistrationEmail(email, token string) error {
	appURL := os.Getenv("FRONTEND_URL")
	if appURL == "" {
		appURL = "http://localhost:3000"
	}
	registerURL := appURL + "/register/confirm?token=" + token
	body := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ja"><head><meta charset="UTF-8"><title>会員登録の確認</title></head>
<body style="font-family:sans-serif;background:#f5f5f5;padding:20px;">
<div style="max-width:500px;margin:0 auto;background:#fff;border-radius:8px;padding:32px;">
<h2 style="color:#1976D2;">会員登録の確認</h2>
<p>ご登録ありがとうございます。</p>
<p>以下のボタンをクリックして会員登録を完了してください。</p>
<a href="%s" style="display:inline-block;background:#1976D2;color:#fff;padding:12px 24px;border-radius:6px;text-decoration:none;font-weight:bold;margin:16px 0;">会員登録を完了する</a>
<p style="color:#888;font-size:12px;">このリンクは24時間有効です。身に覚えのない場合は無視してください。</p>
</div>
</body></html>`, registerURL)

	return s.sendHTML([]string{email}, "会員登録の確認", body)
}

// SendPasswordResetEmail パスワードリセットメールを送信
func (s *EmailService) SendPasswordResetEmail(email, token, appURL string) error {
	resetURL := appURL + "/reset-password?token=" + token
	body := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ja"><head><meta charset="UTF-8"><title>パスワードのリセット</title></head>
<body style="font-family:sans-serif;background:#f5f5f5;padding:20px;">
<div style="max-width:500px;margin:0 auto;background:#fff;border-radius:8px;padding:32px;">
<h2 style="color:#1976D2;">パスワードのリセット</h2>
<p>パスワードのリセットリクエストを受け付けました。</p>
<p>以下のボタンをクリックして新しいパスワードを設定してください。</p>
<a href="%s" style="display:inline-block;background:#1976D2;color:#fff;padding:12px 24px;border-radius:6px;text-decoration:none;font-weight:bold;margin:16px 0;">パスワードをリセットする</a>
<p style="color:#888;font-size:12px;">このリンクは1時間有効です。身に覚えのない場合は無視してください。</p>
</div>
</body></html>`, resetURL)

	return s.sendHTML([]string{email}, "パスワードのリセット", body)
}

// SendInterviewReport 面接練習レポートをメールで送信
func (s *EmailService) SendInterviewReport(user *entity.User, data InterviewReportEmailData) error {
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	data.SentAt = time.Now().In(jst).Format("2006年01月02日 15:04")
	data.UserName = user.Name

	tmpl, err := template.New("interview_report").Parse(interviewReportEmailTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}
	htmlBody := buf.String()
	return s.sendHTML([]string{user.Email}, "AI面接練習レポート", htmlBody)
}

// SendSystemAlertEmail sends a plain-text operational alert email to multiple recipients.
func (s *EmailService) SendSystemAlertEmail(recipients []string, subject, body string) error {
	if len(recipients) == 0 {
		return nil
	}
	return s.sendText(recipients, subject, body)
}

// SendCompanyEntryThankYouAndInvite 企業情報投稿への感謝と本登録依頼メール（#754）
// inviteToken が空の場合はログイン案内のみ（既に会員の場合）。
func (s *EmailService) SendCompanyEntryThankYouAndInvite(email, companyName, inviteToken string) error {
	appURL := os.Getenv("FRONTEND_URL")
	if appURL == "" {
		appURL = "http://localhost:3000"
	}
	companyName = strings.TrimSpace(companyName)
	if companyName == "" {
		companyName = "貴社"
	}

	var ctaBlock string
	if inviteToken != "" {
		registerURL := appURL + "/register/confirm?token=" + inviteToken
		ctaBlock = fmt.Sprintf(`
<p>引き続き、求人票の正式掲載・編集や企業情報の更新を行うには、システムへの会員登録をお願いします。</p>
<a href="%s" style="display:inline-block;background:#1976D2;color:#fff;padding:12px 24px;border-radius:6px;text-decoration:none;font-weight:bold;margin:16px 0;">会員登録を完了する</a>
<p style="color:#888;font-size:12px;">このリンクは24時間有効です。</p>`, registerURL)
	} else {
		loginURL := appURL + "/login"
		ctaBlock = fmt.Sprintf(`
<p>すでにアカウントをお持ちの場合は、ログインのうえ管理画面から掲載状況をご確認ください。</p>
<a href="%s" style="display:inline-block;background:#1976D2;color:#fff;padding:12px 24px;border-radius:6px;text-decoration:none;font-weight:bold;margin:16px 0;">ログインする</a>`, loginURL)
	}

	body := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ja"><head><meta charset="UTF-8"><title>企業情報のご登録ありがとうございます</title></head>
<body style="font-family:sans-serif;background:#f5f5f5;padding:20px;">
<div style="max-width:560px;margin:0 auto;background:#fff;border-radius:8px;padding:32px;">
<h2 style="color:#1976D2;">企業情報のご提供ありがとうございます</h2>
<p>%s の情報を受け付けました。</p>
<p>内容を確認のうえ掲載審査を行います。公開は審査完了後となります（自動公開ではありません）。</p>
%s
<p style="color:#888;font-size:12px;">このメールはAI就活エージェントから自動送信されました。</p>
</div>
</body></html>`, companyName, ctaBlock)

	return s.sendHTML([]string{email}, "【AI就活エージェント】企業情報のご登録ありがとうございます", body)
}

// SendCompanyUserInvite は企業ポータル招待メールを送る（#1091）。
func (s *EmailService) SendCompanyUserInvite(email, companyName, inviteToken string) error {
	appURL := os.Getenv("FRONTEND_URL")
	if appURL == "" {
		appURL = "http://localhost:3000"
	}
	companyName = strings.TrimSpace(companyName)
	if companyName == "" {
		companyName = "貴社"
	}
	setupURL := appURL + "/company-portal/setup?token=" + inviteToken
	body := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ja"><head><meta charset="UTF-8"><title>企業ポータルへのご招待</title></head>
<body style="font-family:sans-serif;background:#f5f5f5;padding:20px;">
<div style="max-width:560px;margin:0 auto;background:#fff;border-radius:8px;padding:32px;">
<h2 style="color:#1976D2;">企業ポータルへのご招待</h2>
<p>%s の採用担当者として、AI就活エージェント企業ポータルへ招待されました。</p>
<p>以下のリンクからパスワードを設定し、ログインしてください。</p>
<a href="%s" style="display:inline-block;background:#1976D2;color:#fff;padding:12px 24px;border-radius:6px;text-decoration:none;font-weight:bold;margin:16px 0;">アカウントを有効化する</a>
<p style="color:#888;font-size:12px;">このリンクは24時間有効です。</p>
</div>
</body></html>`, companyName, setupURL)
	return s.sendHTML([]string{email}, "【AI就活エージェント】企業ポータルへのご招待", body)
}
