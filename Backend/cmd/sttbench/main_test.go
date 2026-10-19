package main

import "testing"

func TestParseHintsContext(t *testing.T) {
	tests := []struct {
		name                                     string
		in                                       string
		wantName, wantReading, wantPos, wantInfo string
	}{
		{name: "空文字は全て空", in: "", wantName: ""},
		{name: "4要素", in: "株式会社サンプル|サンプル|バックエンド|Go と AWS",
			wantName: "株式会社サンプル", wantReading: "サンプル", wantPos: "バックエンド", wantInfo: "Go と AWS"},
		{name: "足りない要素は空", in: "株式会社サンプル|サンプル",
			wantName: "株式会社サンプル", wantReading: "サンプル"},
		// 企業情報は自由記述なので区切り文字が現れうる。余りを捨てると情報が欠ける。
		{name: "5要素目以降は企業情報へ寄せる", in: "A|B|C|D|E",
			wantName: "A", wantReading: "B", wantPos: "C", wantInfo: "D|E"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, reading, pos, info := parseHintsContext(tt.in)
			if name != tt.wantName || reading != tt.wantReading || pos != tt.wantPos || info != tt.wantInfo {
				t.Errorf("parseHintsContext(%q) = (%q,%q,%q,%q), want (%q,%q,%q,%q)",
					tt.in, name, reading, pos, info, tt.wantName, tt.wantReading, tt.wantPos, tt.wantInfo)
			}
		})
	}
}
