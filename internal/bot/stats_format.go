package bot

import (
	"fmt"
	"strings"

	"food-kbju-bot/internal/store"
)

func formatDayTotal(dt *store.DayTotal) string {
	if dt.Meals == 0 {
		return fmt.Sprintf("Сегодня (%s) пока нет сохранённых приёмов пищи.\nПришли фото еды — сохраню автоматически.", dt.Day)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Сегодня (%s)\nПриёмов: %d\n\n", dt.Day, dt.Meals)
	fmt.Fprintf(&b, "К %.0f  Б %.1f  Ж %.1f  У %.1f", dt.Kcal, dt.Protein, dt.Fat, dt.Carbs)
	return b.String()
}

func formatWeekStats(days []store.DayTotal) string {
	var b strings.Builder
	b.WriteString("Среднее за 7 дней (по дням с записями)\n\n")

	var sumK, sumP, sumF, sumC float64
	active := 0
	for _, d := range days {
		mark := "—"
		if d.Meals > 0 {
			active++
			sumK += d.Kcal
			sumP += d.Protein
			sumF += d.Fat
			sumC += d.Carbs
			mark = fmt.Sprintf("К %.0f / Б %.0f / Ж %.0f / У %.0f (%d)", d.Kcal, d.Protein, d.Fat, d.Carbs, d.Meals)
		}
		fmt.Fprintf(&b, "%s: %s\n", d.Day[5:], mark)
	}

	b.WriteString("\n")
	if active == 0 {
		b.WriteString("За неделю записей нет.")
		return b.String()
	}
	n := float64(active)
	fmt.Fprintf(&b, "Среднее в активный день:\nК %.0f  Б %.1f  Ж %.1f  У %.1f\n", sumK/n, sumP/n, sumF/n, sumC/n)
	fmt.Fprintf(&b, "Дней с едой: %d из 7", active)
	return b.String()
}

func formatOverview(ov *store.Overview, lookbackDays int) string {
	if ov.MealCount == 0 {
		return fmt.Sprintf("Статистика за %d дней пустая.\nПришли несколько фото еды — начну собирать данные.", lookbackDays)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Статистика за %d дней\n\n", lookbackDays)
	fmt.Fprintf(&b, "Приёмов: %d\nДней с записями: %d\n\n", ov.MealCount, ov.DayCount)
	fmt.Fprintf(&b, "Среднее в день:\nК %.0f  Б %.1f  Ж %.1f  У %.1f\n\n", ov.AvgKcal, ov.AvgProtein, ov.AvgFat, ov.AvgCarbs)

	totalMacro := ov.AvgProtein*4 + ov.AvgFat*9 + ov.AvgCarbs*4
	if totalMacro > 0 {
		fmt.Fprintf(&b, "Доля макросов (по среднему дню):\n")
		fmt.Fprintf(&b, "Б %.0f%%  Ж %.0f%%  У %.0f%%\n",
			100*ov.AvgProtein*4/totalMacro,
			100*ov.AvgFat*9/totalMacro,
			100*ov.AvgCarbs*4/totalMacro,
		)
	}

	if len(ov.TopFoods) > 0 {
		b.WriteString("\nЧаще всего:\n")
		for _, f := range ov.TopFoods {
			fmt.Fprintf(&b, "• %s — %d\n", f.Name, f.Count)
		}
	}
	return b.String()
}

func overviewPrompt(ov *store.Overview, lookbackDays int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Период: последние %d дней.\n", lookbackDays)
	fmt.Fprintf(&b, "Приёмов пищи: %d, дней с записями: %d.\n", ov.MealCount, ov.DayCount)
	fmt.Fprintf(&b, "Среднее КБЖУ в день: К %.0f, Б %.1f, Ж %.1f, У %.1f.\n", ov.AvgKcal, ov.AvgProtein, ov.AvgFat, ov.AvgCarbs)
	if len(ov.TopFoods) > 0 {
		b.WriteString("Частые блюда: ")
		parts := make([]string, 0, len(ov.TopFoods))
		for _, f := range ov.TopFoods {
			parts = append(parts, fmt.Sprintf("%s (%d)", f.Name, f.Count))
		}
		b.WriteString(strings.Join(parts, ", "))
		b.WriteString(".\n")
	}
	if len(ov.Days) > 0 {
		b.WriteString("По дням:\n")
		for _, d := range ov.Days {
			fmt.Fprintf(&b, "- %s: К %.0f Б %.0f Ж %.0f У %.0f, приёмов %d\n", d.Day, d.Kcal, d.Protein, d.Fat, d.Carbs, d.Meals)
		}
	}
	return b.String()
}
