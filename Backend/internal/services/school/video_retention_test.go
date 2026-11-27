package school

import "testing"

func TestAllowedInterviewVideoRetention(t *testing.T) {
	days := 90
	if !AllowedInterviewVideoRetention(&days) || !AllowedInterviewVideoRetention(nil) {
		t.Fatal("90 と期限なしは認める")
	}
	bad := 45
	if AllowedInterviewVideoRetention(&bad) {
		t.Fatal("45日は認めない")
	}
}
