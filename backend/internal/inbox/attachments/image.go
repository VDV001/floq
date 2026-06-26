package attachments

import (
	"context"
	"fmt"
)

// imageOCRPrompt asks the vision model to transcribe verbatim plus
// describe layout. Russian-only response so it slots straight into the
// qualification context the AI sees alongside the email body. The
// prompt deliberately bans markdown and preamble — both interfere with
// downstream prompt building.
const imageOCRPrompt = `Извлеки весь видимый текст из изображения дословно, без интерпретаций.
Если изображение содержит интерфейс или скриншот — перечисли видимые элементы (заголовки, кнопки, поля, сообщения об ошибках).
Если читаемого текста нет — дай короткое описание того, что изображено (1-2 предложения).
Только результат, без преамбулы, без markdown.`

// extractImageText delegates to the injected VisionClient. Returning a
// wrapped error rather than ErrUnsupportedFormat lets the analyser
// distinguish "provider error" (skippable, log + continue) from
// "format not understood" (also skippable but a different log line).
func extractImageText(ctx context.Context, vc VisionClient, data []byte, mimeType string) (string, error) {
	if vc == nil {
		return "", ErrUnsupportedFormat
	}
	text, err := vc.AnalyzeImage(ctx, data, mimeType, imageOCRPrompt)
	if err != nil {
		return "", fmt.Errorf("analyze image: %w", err)
	}
	return text, nil
}
