package store

import (
	"path/filepath"
	"testing"
	"time"

	"food-kbju-bot/internal/kbju"
)

func TestAddAndDayTotal(t *testing.T) {
	dir := t.TempDir()
	loc := time.FixedZone("MSK", 3*3600)
	st, err := Open(filepath.Join(dir, "t.db"), loc)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	r := &kbju.Result{
		Items: []kbju.Item{{Name: "Омлет", Grams: 150, Kcal: 200, Protein: 14, Fat: 15, Carbs: 2}},
		Total: kbju.Nutrients{Kcal: 200, Protein: 14, Fat: 15, Carbs: 2},
	}
	id, err := st.AddMeal(42, r)
	if err != nil || id == 0 {
		t.Fatalf("add: id=%d err=%v", id, err)
	}

	day, err := st.DayTotal(42, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if day.Meals != 1 || day.Kcal != 200 {
		t.Fatalf("unexpected day: %+v", day)
	}

	r.Total.Kcal = 250
	if err := st.UpdateMeal(id, 42, r); err != nil {
		t.Fatal(err)
	}
	day, _ = st.DayTotal(42, time.Now())
	if day.Kcal != 250 {
		t.Fatalf("update failed: %+v", day)
	}
}
