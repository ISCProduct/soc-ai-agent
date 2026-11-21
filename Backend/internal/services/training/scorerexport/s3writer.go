package scorerexport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Writer は出力を S3 に書く ObjectStore。
// 資格情報は AWS SDK の既定の順（環境変数・実行ロール）で読む。書き込みの権限は
// 書き出しジョブの実行ロールだけに付ける想定（IAM の設定は infra の別 PR）。
// 退会のパージを行うサーバーには、一覧・マニフェストの読み書き・学生のファイルの削除だけを付ける
// （学生のファイルは読まない。sweep.go の ObjectStore）。
type S3Writer struct {
	bucket string
	client *s3.Client
}

var _ ObjectStore = (*S3Writer)(nil)

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

// ListKeys は prefix で始まるキーをすべて返す（1000件ずつのページをたどる）。
func (w *S3Writer) ListKeys(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	p := s3.NewListObjectsV2Paginator(w.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(w.bucket),
		Prefix: aws.String(prefix),
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("s3 list: %w", err)
		}
		for _, o := range page.Contents {
			keys = append(keys, aws.ToString(o.Key))
		}
	}
	return keys, nil
}

// GetObject は key の中身を返す。無ければ ErrObjectNotFound。マニフェストを読むのに使う。
func (w *S3Writer) GetObject(ctx context.Context, key string) ([]byte, error) {
	out, err := w.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(w.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("s3 get: %w", err)
	}
	defer out.Body.Close()
	body, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("s3 get: %w", err)
	}
	return body, nil
}

// DeleteObject は key のオブジェクトを消す。存在しなくてもエラーにしない（S3 の DeleteObject と同じ）。
func (w *S3Writer) DeleteObject(ctx context.Context, key string) error {
	_, err := w.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(w.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// PutObject と同じく、キー（subject を含む）はエラーに含めない。
		return fmt.Errorf("s3 delete: %w", err)
	}
	return nil
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
