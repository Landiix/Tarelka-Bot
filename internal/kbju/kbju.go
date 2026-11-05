package kbju

import (
	"fmt"
	"strings"
)

type Nutrients struct {
	Kcal     float64 `json:"kcal"`
	Protein  float64 `json:"protein"`
	Fat      float64 `json:"fat"`
	Carbs    float64 `json:"carbs"`
}

type Item struct {
	Name    string  `json:"name"`
	Grams   float64 `json:"grams"`
	Kcal    float64 `json:"kcal"`
	Protein float64 `json:"protein"`
	Fat     float64 `json:"fat"`
	Carbs   float64 `json:"carbs"`
}

type Result struct {
	Items      []Item    `json:"items"`
	Total      Nutrients `json:"total"`
	Confidence string    `json:"confidence"`
	Notes      string    `json:"notes"`
}

func FormatReply(r *Result) string {
	var b strings.Builder

	b.WriteString("Примерно (оценка по фото):\n\n")

	if len(r.Items) > 0 {
		for _, it := range r.Items {
			fmt.Fprintf(&b, "• %s — ~%.0f г\n", it.Name, it.Grams)
			fmt.Fprintf(&b, "  К %.0f / Б %.1f / Ж %.1f / У %.1f\n", it.Kcal, it.Protein, it.Fat, it.Carbs)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "Итого:\nК %.0f  Б %.1f  Ж %.1f  У %.1f\n",
		r.Total.Kcal, r.Total.Protein, r.Total.Fat, r.Total.Carbs)

	if r.Confidence != "" {
		fmt.Fprintf(&b, "\nУверенность: %s", confidenceRU(r.Confidence))
	}
	if strings.TrimSpace(r.Notes) != "" {
		fmt.Fprintf(&b, "\nЗаметки: %s", strings.TrimSpace(r.Notes))
	}

	b.WriteString("\n\nМожно уточнить текстом, например: «это 300 г пасты» — пересчитаю и обновлю дневник.")
	return b.String()
}

func confidenceRU(c string) string {
	switch strings.ToLower(strings.TrimSpace(c)) {
	case "high":
		return "высокая"
	case "medium":
		return "средняя"
	case "low":
		return "низкая"
	default:
		return c
	}
}
