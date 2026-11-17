package scorerexport

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Writer は出力を S3 に書く ObjectWriter。
// 資格情報は AWS SDK の既定の順（環境変数・実行ロール）で読む。書き込みの権限は
// 書き出しジョブの実行ロールだけに付ける想定（IAM の設定は infra の別 PR）。
type S3Writer struct {
	bucket string
	client *s3.Client
}

var _ ObjectWriter = (*S3Writer)(nil)

// NewS3Writer は bucket に書く S3Writer を作る。リージョンは AWS_REGION（未設定なら ap-northeast-1。
// storage.NewS3UploadService と同じ既定）。
func NewS3Writer(ctx context.Context, bucket string) (*S3Writer, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "ap-northeast-1"
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("scorerexport: AWS の設定を読めませんでした: %w", err)
	}
	return &S3Writer{bucket: bucket, client: s3.NewFromConfig(cfg)}, nil
}

// PutObject は body を key に書く。サーバー側の暗号化を必ず指定する（バケットの既定の設定に頼らない）。
// 公開の ACL は付けない。
func (w *S3Writer) PutObject(ctx context.Context, key string, body []byte, contentType string) error {
	_, err := w.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(w.bucket),
		Key:                  aws.String(key),
		Body:                 bytes.NewReader(body),
		ContentType:          aws.String(contentType),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	})
	if err != nil {
		// キーは含めない。キーには subject（利用者 ID の HMAC）が入り、エラーは監査ログに残るため。
		// SDK のエラー自体がキーを含む場合（AccessDenied の resource など）は Job.Run の redactHashes で伏せる。
		return fmt.Errorf("s3 put: %w", err)
	}
	return nil
}
