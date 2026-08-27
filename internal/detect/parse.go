package detect

import (
	"encoding/json"
	"strings"
)

// ParseQuestionReply interprets a small-model yes/no or JSON response.
// Garbage or unknown text is treated as not a question.
func ParseQuestionReply(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return false
	}

	// Try JSON: {"question": true} or {"is_question": true}
	var obj map[string]any
	if err := json.Unmarshal([]byte(s), &obj); err == nil {
		for _, key := range []string{"question", "is_question", "isQuestion"} {
			if v, ok := obj[key]; ok {
				switch t := v.(type) {
				case bool:
					return t
				case string:
					return parseYesNo(t)
				}
			}
		}
	}

	// Strip markdown fences / noise
	lower := strings.ToLower(s)
	lower = strings.Trim(lower, "`\"' \n\t")
	if i := strings.Index(lower, "{"); i >= 0 {
		if j := strings.LastIndex(lower, "}"); j > i {
			var obj2 map[string]any
			if err := json.Unmarshal([]byte(lower[i:j+1]), &obj2); err == nil {
				if v, ok := obj2["question"]; ok {
					if b, ok := v.(bool); ok {
						return b
					}
				}
			}
		}
	}
	return parseYesNo(lower)
}

func parseYesNo(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.Trim(s, "`\"'.!")
	switch {
	case s == "yes" || s == "y" || s == "true" || s == "question":
		return true
	case strings.HasPrefix(s, "yes"):
		return true
	case s == "no" || s == "n" || s == "false" || s == "not a question" || s == "statement":
		return false
	default:
		return false
	}
}

// QuestionPrompt is the system/user prompt for the local gate model.
func QuestionPrompt(text string) (system, user string) {
	system = `You classify meeting utterances. Reply with JSON only: {"question":true} or {"question":false}.
A question asks for information, a decision, confirmation, or an answer from someone.
Statements, acknowledgements, and filler are not questions.`
	user = text
	return system, user
}
