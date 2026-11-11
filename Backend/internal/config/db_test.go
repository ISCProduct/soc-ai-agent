package config

import "testing"

func TestPositiveIntEnv(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int
	}{
		{name: "未設定は既定値", value: "", want: 30},
		{name: "正の値を使う", value: "12", want: 12},
		{name: "0は既定値", value: "0", want: 30},
		{name: "負の値は既定値", value: "-1", want: 30},
		{name: "数値でなければ既定値", value: "many", want: 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DB_MAX_OPEN_CONNS", tt.value)
			if got := positiveIntEnv("DB_MAX_OPEN_CONNS", 30); got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}
