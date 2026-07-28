package services

import "testing"

func TestIsEngineerPosition(t *testing.T) {
	tests := []struct {
		name     string
		position string
		want     bool
	}{
		{name: "日本語エンジニア", position: "バックエンドエンジニア", want: true},
		{name: "英語小文字", position: "software engineer", want: true},
		{name: "英語大文字", position: "Senior Developer", want: true},
		{name: "SRE", position: "Site Reliability Engineer (SRE)", want: true},
		{name: "非エンジニア", position: "法人営業", want: false},
		{name: "空文字", position: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEngineerPosition(tt.position); got != tt.want {
				t.Fatalf("isEngineerPosition(%q) = %v, want %v", tt.position, got, tt.want)
			}
		})
	}
}
