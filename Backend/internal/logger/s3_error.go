package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

// ObjectPutter はエラーログ JSON をオブジェクトストレージへ置く。
type ObjectPutter interface {
	UploadFile(ctx context.Context, key, mimeType string, data []byte) (string, string, error)
}

var s3ErrorPutter ObjectPutter

// EnableS3ErrorArchive は stg/prod 向けエラーログの保存先を注入する。
func EnableS3ErrorArchive(p ObjectPutter) {
	s3ErrorPutter = p
}

func archiveEnv() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))) {
	case "staging", "stg":
		return "staging"
	case "production", "prod":
		return "production"
	case "development", "dev", "local", "test":
		return ""
	default:
		// staging EC2 は歴史的に APP_ENV 未設定。S3 バケットがあるときだけ stg 扱い。
		if strings.TrimSpace(os.Getenv("APP_ENV")) == "" && s3BucketConfigured() {
			return "staging"
		}
		return ""
	}
}

func s3BucketConfigured() bool {
	return strings.TrimSpace(os.Getenv("AWS_S3_BUCKET")) != "" || strings.TrimSpace(os.Getenv("S3_BUCKET")) != ""
}

func s3ErrorArchiveEnabled() bool {
	return s3ErrorPutter != nil && archiveEnv() != ""
}

func sanitizeKind(kind string) string {
	if kind == "" {
		return "error"
	}
	var b strings.Builder
	for _, r := range kind {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" {
		return "error"
	}
	return s
}

func errorLogKey(kind string, now time.Time) string {
	env := archiveEnv()
	if env == "" {
		env = "unknown"
	}
	return fmt.Sprintf("error-logs/%s/%s/%s-%d.json",
		env, now.UTC().Format("2006/01/02"), sanitizeKind(kind), now.UTC().UnixNano())
}

// PutErrorJSON は stg/prod のみ S3 にエラー JSON を置く。ローカルは何もしない。
// 戻り値は置く予定のキー（無効時は空）。アップロードは待たない。
func PutErrorJSON(kind string, payload any) string {
	if !s3ErrorArchiveEnabled() {
		return ""
	}
	now := time.Now()
	key := errorLogKey(kind, now)
	doc, err := json.Marshal(map[string]any{
		"time": now.UTC().Format(time.RFC3339Nano),
		"env":  archiveEnv(),
		"kind": kind,
		"data": payload,
	})
	if err != nil {
		slog.Warn("s3 error log marshal failed", "kind", kind, "error", err)
		return ""
	}
	putter := s3ErrorPutter
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, _, err := putter.UploadFile(ctx, key, "application/json", doc); err != nil {
			slog.Warn("s3 error log upload failed", "key", key, "error", err)
			return
		}
		slog.Info("s3 error log uploaded", "key", key)
	}()
	return key
}
