package domain

import "strings"

// DetectCallAgreement checks if the message indicates the person agrees to a call/meeting.
// This is a domain service for detecting intent from message text.
func DetectCallAgreement(text string) bool {
	lower := strings.ToLower(text)
	markers := []string{
		"давайте созвон", "давай созвон", "готов созвон", "согласен на созвон",
		"можно созвон", "давайте звонок", "давай звонок", "готов к звонку",
		"давайте встреч", "давай встреч", "согласен на встреч", "готов встретить",
		"можем созвон", "можем встретить", "давайте обсудим", "готов обсудить",
		"да, давайте", "да давайте", "конечно, давайте", "с удовольствием",
		"когда удобно", "выберу время", "забронир", "запишусь",
		"да, можно", "да можно", "ок, давай", "ок давай",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}
