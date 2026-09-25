package resume

import "testing"

// 取得先サーバーが名乗るファイル名で保存先の外へ出られないことを固定する。
// filepath.Base を外すと1件目が /etc/cron.d/x へ着弾する。
func TestDownloadFilename(t *testing.T) {
	tests := []struct {
		name        string
		disposition string
		urlPath     string
		want        string
	}{
		{"親ディレクトリへの脱出", `attachment; filename="../../../../etc/cron.d/x"`, "/a.pdf", "x.pdf"},
		{"絶対パス", `attachment; filename="/etc/passwd"`, "/a.pdf", "passwd.pdf"},
		{"Windows区切り", `attachment; filename="..\..\boot.ini"`, "/a.pdf", "boot.ini"},
		{"..だけ", `attachment; filename=".."`, "/a.pdf", "document.pdf"},
		{"通常のファイル名", `attachment; filename="resume.pdf"`, "/a.pdf", "resume.pdf"},
		{"拡張子なしはpdfを補う", `attachment; filename="resume"`, "/a.pdf", "resume.pdf"},
		{"ヘッダー無しはURLから", "", "/files/cv.docx", "cv.docx"},
		{"URLもディレクトリだけ", "", "/files/", "files.pdf"},
		{"URLが空", "", "", "document.pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := downloadFilename(tt.disposition, tt.urlPath); got != tt.want {
				t.Errorf("downloadFilename(%q, %q) = %q, want %q", tt.disposition, tt.urlPath, got, tt.want)
			}
		})
	}
}
