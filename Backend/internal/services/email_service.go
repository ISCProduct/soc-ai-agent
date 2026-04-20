package services

import (
	"Backend/domain/entity"
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"os"
	"strconv"
	"time"
)

// EmailService はSMTPを介してメール送信を担うサービス。
// SMTP_HOST 環境変数が未設定の場合は開発用フォールバックとしてログ出力のみを行う。
type EmailService struct {
	host     string
	port     int
	user     string
	password string
	from     string
}

// NewEmailService は環境変数からSMTP設定を読み込み EmailService を生成する。
// SMTP_HOST が空の場合は実送信を行わず、標準出力へのログ出力のみで動作する（開発環境フォールバック）。
// SMTP_PORT が未設定または不正値の場合はデフォルト 587 ポートを使用する。
func NewEmailService() *EmailService {
	port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if port == 0 {
		port = 587
	}
	return &EmailService{
		host:     os.Getenv("SMTP_HOST"),
		port:     port,
		user:     os.Getenv("SMTP_USER"),
		password: os.Getenv("SMTP_PASSWORD"),
		from:     os.Getenv("SMTP_FROM"),
	}
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
    <h2>📊 4分析スコア</h2>
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
func (s *EmailService) SendAnalysisReport(user *entity.User, summary *AnalysisSummary, companies []EmailReportCompany, sessionID string) error {
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

	// SMTP未設定の場合はログ出力のみ（開発環境向け）
	if s.host == "" {
		log.Printf("[EmailService] SMTP not configured. Simulating send to %s (body: %d bytes)\n", user.Email, len(htmlBody))
		return nil
	}

	// MIMEヘッダーとHTMLボディを組み立ててSMTP送信
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: AI就活分析レポート\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		s.from, user.Email, htmlBody,
	)
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	auth := smtp.PlainAuth("", s.user, s.password, s.host)

	if err := smtp.SendMail(addr, auth, s.from, []string{user.Email}, []byte(msg)); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("[EmailService] Email sent successfully to %s\n", user.Email)
	return nil
}

// SendVerificationEmail メール認証用のメールを送信
func (s *EmailService) SendVerificationEmail(user *entity.User, token, appURL string) error {
	verifyURL := appURL + "/verify-email?token=" + token
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
</body></html>`, user.Name, verifyURL)

	if s.host == "" {
		log.Printf("[EmailService] Verification email for %s: %s\n", user.Email, verifyURL)
		return nil
	}

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: メールアドレスの確認\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		s.from, user.Email, body,
	)
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	auth := smtp.PlainAuth("", s.user, s.password, s.host)
	return smtp.SendMail(addr, auth, s.from, []string{user.Email}, []byte(msg))
}

// SendReVerificationEmail 再認証メールを送信
func (s *EmailService) SendReVerificationEmail(user *entity.User, token, appURL string) error {
	verifyURL := appURL + "/verify-email?token=" + token
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
</body></html>`, user.Name, verifyURL)

	if s.host == "" {
		log.Printf("[EmailService] Re-verification email for %s: %s\n", user.Email, verifyURL)
		return nil
	}

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: ログイン再認証のお願い\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		s.from, user.Email, body,
	)
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	auth := smtp.PlainAuth("", s.user, s.password, s.host)
	return smtp.SendMail(addr, auth, s.from, []string{user.Email}, []byte(msg))
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

	if s.host == "" {
		log.Printf("[EmailService] Registration email for %s: %s\n", email, registerURL)
		return nil
	}

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: 会員登録の確認\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		s.from, email, body,
	)
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	auth := smtp.PlainAuth("", s.user, s.password, s.host)
	return smtp.SendMail(addr, auth, s.from, []string{email}, []byte(msg))
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

	if s.host == "" {
		log.Printf("[EmailService] Password reset email for %s: %s\n", email, resetURL)
		return nil
	}

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: パスワードのリセット\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		s.from, email, body,
	)
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	auth := smtp.PlainAuth("", s.user, s.password, s.host)
	return smtp.SendMail(addr, auth, s.from, []string{email}, []byte(msg))
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

	if s.host == "" {
		log.Printf("[EmailService] SMTP not configured. Simulating interview report send to %s (body: %d bytes)\n", user.Email, len(htmlBody))
		return nil
	}

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: AI面接練習レポート\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		s.from, user.Email, htmlBody,
	)
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	auth := smtp.PlainAuth("", s.user, s.password, s.host)
	if err := smtp.SendMail(addr, auth, s.from, []string{user.Email}, []byte(msg)); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	log.Printf("[EmailService] Interview report email sent successfully to %s\n", user.Email)
	return nil
}

// SendSystemAlertEmail sends a plain-text operational alert email to multiple recipients.
func (s *EmailService) SendSystemAlertEmail(recipients []string, subject, body string) error {
	if len(recipients) == 0 {
		return nil
	}
	if s.host == "" || s.user == "" || s.password == "" || s.from == "" {
		log.Printf("[EmailService] SMTP not configured. Simulating alert send to %v\n", recipients)
		return nil
	}

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	auth := smtp.PlainAuth("", s.user, s.password, s.host)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.from, recipients[0], subject, body)
	if err := smtp.SendMail(addr, auth, s.from, recipients, []byte(msg)); err != nil {
		return fmt.Errorf("failed to send alert email: %w", err)
	}
	return nil
}
