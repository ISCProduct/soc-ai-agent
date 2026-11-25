package companyfetch

import "testing"

func TestRobotsPathAllowed(t *testing.T) {
	tests := []struct {
		name, robots, path string
		want               bool
	}{
		{name: "規則なしは許可", robots: "", path: "/company/", want: true},
		{name: "全面禁止", robots: "User-agent: *\nDisallow: /\n", path: "/company/", want: false},
		{name: "前方一致で禁止", robots: "User-agent: *\nDisallow: /company\n", path: "/company/about", want: false},
		{name: "一致しなければ許可", robots: "User-agent: *\nDisallow: /private/\n", path: "/company/", want: true},
		{name: "長く一致した Allow が優先", robots: "User-agent: *\nDisallow: /\nAllow: /company/\n", path: "/company/", want: true},
		{name: "空の Disallow は全許可", robots: "User-agent: *\nDisallow:\n", path: "/", want: true},
		{name: "他のボット向けの規則は見ない", robots: "User-agent: Googlebot\nDisallow: /\n", path: "/", want: true},
		{name: "連続する User-agent は同じグループ", robots: "User-agent: Googlebot\nUser-agent: *\nDisallow: /x\n", path: "/x", want: false},
		{name: "コメントは無視", robots: "User-agent: * # all\nDisallow: /x # no\n", path: "/x", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := robotsPathAllowed(tt.robots, tt.path); got != tt.want {
				t.Errorf("robotsPathAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}
