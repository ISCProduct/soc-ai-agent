package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// #2005: S3 へは実際にアップロードしない。設定不足時のエラー・URL の組み立て・
// 署名付き URL（手元で計算するだけ）と、ローカルのテスト用サーバーへのアップロードを確かめる。

// isolateAWSEnv は、手元の AWS 設定や EC2 のメタデータに触れないようにする。
// LocalStack などのカスタムエンドポイント（AWS_ENDPOINT_URL*）、FIPS・デュアルスタックの指定も無視させる。
// 設定された環境では署名付き URL の宛先（ホスト）が変わり、AWS のホストを前提にした検証が落ちる（#2148 のレビュー）。
func isolateAWSEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AWS_ENDPOINT_URL", "")
	t.Setenv("AWS_ENDPOINT_URL_S3", "")
	t.Setenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS", "true")
	t.Setenv("AWS_USE_FIPS_ENDPOINT", "false")
	t.Setenv("AWS_USE_DUALSTACK_ENDPOINT", "false")
	t.Setenv("AWS_CONFIG_FILE", t.TempDir()+"/none")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", t.TempDir()+"/none")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAFAKEFORTEST")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "fake-secret")
	t.Setenv("AWS_SESSION_TOKEN", "")
}

func TestNewS3UploadService_Config(t *testing.T) {
	cases := []struct {
		name                      string
		bucket, legacyBucket, reg string
		wantErr                   string
		wantBucket, wantRegion    string
	}{
		{name: "バケット未設定はエラー", wantErr: "AWS_S3_BUCKET is not set"},
		{name: "リージョン未設定は東京", bucket: "b1", wantBucket: "b1", wantRegion: "ap-northeast-1"},
		{name: "指定したリージョンを使う", bucket: "b1", reg: "us-west-2", wantBucket: "b1", wantRegion: "us-west-2"},
		{name: "旧名の S3_BUCKET も読む", legacyBucket: "legacy", wantBucket: "legacy", wantRegion: "ap-northeast-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateAWSEnv(t)
			t.Setenv("AWS_S3_BUCKET", tc.bucket)
			t.Setenv("S3_BUCKET", tc.legacyBucket)
			t.Setenv("AWS_REGION", tc.reg)

			svc, err := NewS3UploadService()

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || svc != nil {
					t.Fatalf("svc=%v err=%v, want %q", svc, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if svc.bucket != tc.wantBucket || svc.region != tc.wantRegion || svc.client == nil || svc.uploader == nil {
				t.Errorf("svc = bucket %q region %q client %v uploader %v", svc.bucket, svc.region, svc.client != nil, svc.uploader != nil)
			}
			// フィールドに入れただけでなく、実際の S3 クライアントにリージョンが効いていること（#2148 のレビュー）
			if got := svc.client.Options().Region; got != tc.wantRegion {
				t.Errorf("S3 クライアントのリージョン = %q, want %q", got, tc.wantRegion)
			}
		})
	}
}

// 署名付き URL は手元で計算するだけで、通信しない。
func TestPresignGetURL(t *testing.T) {
	isolateAWSEnv(t)
	t.Setenv("AWS_S3_BUCKET", "videos-bucket")
	t.Setenv("S3_BUCKET", "")
	t.Setenv("AWS_REGION", "ap-northeast-1")
	svc, err := NewS3UploadService()
	if err != nil {
		t.Fatalf("NewS3UploadService: %v", err)
	}

	raw, err := svc.PresignGetURL(context.Background(), "interview-videos/1/a.webm", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignGetURL: %v", err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url: %v", err)
	}
	if !strings.HasPrefix(u.Host, "videos-bucket.s3.ap-northeast-1.") || u.Path != "/interview-videos/1/a.webm" {
		t.Errorf("URL の宛先が違う: %s", raw)
	}
	q := u.Query()
	if q.Get("X-Amz-Expires") != "900" || q.Get("response-content-disposition") != "inline" || q.Get("X-Amz-Signature") == "" {
		t.Errorf("署名付き URL のパラメータが違う: %s", raw)
	}
}

func TestDeleteObject_NotConfigured(t *testing.T) {
	var nilSvc *S3UploadService
	for name, svc := range map[string]*S3UploadService{"nil": nilSvc, "client 無し": {bucket: "b"}} {
		if err := svc.DeleteObject(context.Background(), "k"); err == nil || !strings.Contains(err.Error(), "not configured") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

type s3Request struct {
	method, path, contentType, body string
}

// newLocalS3Service は、S3 の代わりにローカルのテスト用サーバーへ送るサービスを返す。
func newLocalS3Service(t *testing.T, status int, respBody string) (*S3UploadService, *[]s3Request) {
	t.Helper()
	var mu sync.Mutex
	var got []s3Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, s3Request{r.Method, r.URL.Path, r.Header.Get("Content-Type"), string(b)})
		mu.Unlock()
		w.Header().Set("ETag", `"etag"`)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, respBody)
	}))
	t.Cleanup(srv.Close)

	client := s3.New(s3.Options{
		Region:           "ap-northeast-1",
		BaseEndpoint:     aws.String(srv.URL),
		UsePathStyle:     true,
		RetryMaxAttempts: 1,
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "AKIAFAKEFORTEST", SecretAccessKey: "fake-secret"}, nil
		}),
	})
	return &S3UploadService{bucket: "b1", region: "ap-northeast-1", client: client, uploader: transfermanager.New(client)}, &got
}

func TestUploadFile_ReturnsKeyAndURL(t *testing.T) {
	svc, got := newLocalS3Service(t, http.StatusOK, "")

	key, publicURL, err := svc.UploadFile(context.Background(), "resumes/9/doc.pdf", "application/pdf", []byte("%PDF-1.4"))

	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if key != "resumes/9/doc.pdf" || publicURL != "https://b1.s3.ap-northeast-1.amazonaws.com/resumes/9/doc.pdf" {
		t.Errorf("key=%q url=%q", key, publicURL)
	}
	if len(*got) != 1 {
		t.Fatalf("リクエスト数 = %d", len(*got))
	}
	r := (*got)[0]
	// 本文は完全一致ではなく Contains で見る。SDK はチェックサム付きの chunked 形式で送ることがあり、
	// その枠の付き方は SDK のバージョンで変わりうるため（中身が含まれていることだけを確かめる）。
	if r.method != http.MethodPut || r.path != "/b1/resumes/9/doc.pdf" || r.contentType != "application/pdf" || !strings.Contains(r.body, "%PDF-1.4") {
		t.Errorf("送ったリクエスト = %+v", r)
	}
}

func TestUploadFile_ErrorFromS3(t *testing.T) {
	svc, _ := newLocalS3Service(t, http.StatusForbidden,
		`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`)

	key, publicURL, err := svc.UploadFile(context.Background(), "k", "text/plain", []byte("x"))

	if err == nil || !strings.Contains(err.Error(), "s3 upload") {
		t.Fatalf("err = %v", err)
	}
	if key != "" || publicURL != "" {
		t.Errorf("失敗したのに key/url を返した: %q %q", key, publicURL)
	}
}

// 退会時の PII 削除で使う。成功時は対象のキーへ DELETE を送り、失敗はエラーとして返す（#2148 のレビュー）。
func TestDeleteObject(t *testing.T) {
	t.Run("成功", func(t *testing.T) {
		svc, got := newLocalS3Service(t, http.StatusNoContent, "")
		if err := svc.DeleteObject(context.Background(), "interview-videos/1/a.webm"); err != nil {
			t.Fatalf("DeleteObject: %v", err)
		}
		if len(*got) != 1 || (*got)[0].method != http.MethodDelete || (*got)[0].path != "/b1/interview-videos/1/a.webm" {
			t.Errorf("送ったリクエスト = %+v", *got)
		}
	})
	t.Run("S3 のエラー", func(t *testing.T) {
		svc, _ := newLocalS3Service(t, http.StatusForbidden,
			`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`)
		if err := svc.DeleteObject(context.Background(), "k"); err == nil || !strings.Contains(err.Error(), "s3 delete") {
			t.Fatalf("err = %v", err)
		}
	})
}
