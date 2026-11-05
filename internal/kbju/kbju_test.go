package kbju

import (
	"strings"
	"testing"
)

func TestFormatReply(t *testing.T) {
	r := &Result{
		Items: []Item{
			{Name: "омлет", Grams: 180, Kcal: 250, Protein: 18, Fat: 18, Carbs: 3},
		},
		Total:      Nutrients{Kcal: 250, Protein: 18, Fat: 18, Carbs: 3},
		Confidence: "medium",
		Notes:      "на глаз",
	}

	out := FormatReply(r)
	for _, want := range []string{"омлет", "Итого", "средняя", "на глаз", "300 г"} {
		if !strings.Contains(out, want) {
			t.Fatalf("FormatReply missing %q in:\n%s", want, out)
		}
	}
}
