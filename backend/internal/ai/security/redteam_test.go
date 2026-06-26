package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// redteamFixture mirrors the structure of tests/redteam.yaml. Only
// the fields we assert on are decoded; rest are ignored by yaml.v3.
type redteamFixture struct {
	Tests []struct {
		Description string `yaml:"description"`
		Vars        struct {
			Payload         string `yaml:"payload"`
			Language        string `yaml:"language"`
			ExpectedVerdict string `yaml:"expected_verdict"`
			ExpectedPattern string `yaml:"expected_pattern"`
		} `yaml:"vars"`
	} `yaml:"tests"`
}

// TestInputFirewall_RedteamCorpus loads the canonical YAML redteam
// dataset and verifies each scenario produces the expected verdict.
// This is the CI gate: failure here blocks merge into main.
//
// Updating the dataset: edit tests/redteam.yaml. Each scenario is a
// failure-tested scenario — if you add a new pattern to InputFirewall,
// add at least one positive (block) and one negative (info) scenario
// to keep the false-positive rate under control.
func TestInputFirewall_RedteamCorpus(t *testing.T) {
	// Walk up from cwd to find tests/redteam.yaml — test runs from
	// backend/internal/ai/security but the fixture lives at repo root.
	path := findFixture(t, "tests/redteam.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "read redteam fixture")

	var fx redteamFixture
	require.NoError(t, yaml.Unmarshal(data, &fx), "parse redteam yaml")
	require.GreaterOrEqual(t, len(fx.Tests), 30, "redteam corpus must have >=30 scenarios (pilot floor)")

	f := NewInputFirewall()

	for _, tc := range fx.Tests {
		t.Run(tc.Description, func(t *testing.T) {
			r := f.Scan(tc.Vars.Payload)
			expected := strings.ToLower(strings.TrimSpace(tc.Vars.ExpectedVerdict))

			switch expected {
			case "block":
				assert.False(t, r.Allowed, "payload should be blocked: %q", tc.Vars.Payload)
				assert.Equal(t, SeverityBlock, r.Severity)
			case "warn":
				assert.True(t, r.Allowed, "payload should pass with warn: %q", tc.Vars.Payload)
				assert.Equal(t, SeverityWarn, r.Severity)
			case "info":
				assert.True(t, r.Allowed, "payload should pass clean (info): %q", tc.Vars.Payload)
				assert.Equal(t, SeverityInfo, r.Severity, "matched: %v", r.MatchedPatterns)
			default:
				t.Fatalf("unknown expected_verdict %q in scenario %q", expected, tc.Description)
			}

			if tc.Vars.ExpectedPattern != "" {
				assert.Contains(t, r.MatchedPatterns, tc.Vars.ExpectedPattern,
					"expected pattern %q to match for payload %q (got %v)",
					tc.Vars.ExpectedPattern, tc.Vars.Payload, r.MatchedPatterns)
			}
		})
	}
}

// findFixture walks up from cwd until it finds the named file. Tests
// run with cwd = package dir; the YAML is at repo root.
func findFixture(t *testing.T, rel string) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find %s walking up from cwd", rel)
	return ""
}
