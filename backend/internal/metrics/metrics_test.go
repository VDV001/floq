package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daniil/floq/internal/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_ExposesGoAndProcessCollectors(t *testing.T) {
	m := metrics.New()

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	out := string(body)

	// Runtime + process collectors must be wired into the same registry.
	assert.True(t, strings.Contains(out, "go_goroutines"), "Go runtime collector must be registered")
	assert.True(t, strings.Contains(out, "process_"), "process collector must be registered")
}
