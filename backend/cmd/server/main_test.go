package main

import "testing"

// TestVerifySecretsExitCode pins the safety gate: any secret still needing
// rotation (bad>0 in either module) must yield a non-zero exit so the
// `-verify-secrets-kek` runbook step blocks removal of FLOQ_SECRETS_KEK_OLD.
func TestVerifySecretsExitCode(t *testing.T) {
	cases := []struct {
		name             string
		settingsBad      int
		onecBad          int
		want             int
	}{
		{"all rotated", 0, 0, 0},
		{"settings straggler", 1, 0, 1},
		{"onec straggler", 0, 1, 1},
		{"both", 3, 2, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := verifySecretsExitCode(c.settingsBad, c.onecBad); got != c.want {
				t.Errorf("verifySecretsExitCode(%d, %d) = %d, want %d", c.settingsBad, c.onecBad, got, c.want)
			}
		})
	}
}
