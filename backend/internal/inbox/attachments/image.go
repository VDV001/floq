package attachments

import "context"

// extractImageText is the RED stub. The GREEN commit calls
// vc.AnalyzeImage with a Russian-language OCR prompt and wraps any
// provider error with SkipVisionError on the Result.
func extractImageText(_ context.Context, _ VisionClient, _ []byte, _ string) (string, error) {
	return "", ErrUnsupportedFormat
}
