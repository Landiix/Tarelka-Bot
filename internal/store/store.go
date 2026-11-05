package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"food-kbju-bot/internal/kbju"

	_ "modernc.org/sqlite"
)

type Meal struct {
	ID         int64
	UserID     int64
	CreatedAt  time.Time
	Items      []kbju.Item
	Kcal       float64
	Protein    float64
	Fat        float64
	Carbs      float64
	Notes      string
	Confidence string
}

type DayTotal struct {
	Day     string // YYYY-MM-DD in user TZ
	Meals   int
	Kcal    float64
	Protein float64
	Fat     float64
	Carbs   float64
}

type FoodCount struct {
	Name  string
	Count int
}

type Overview struct {
	MealCount   int
	DayCount    int
	AvgKcal     float64
	AvgProtein  float64
	AvgFat      float64
	AvgCarbs    float64
	TotalKcal   float64
	TotalProtein float64
	TotalFat    float64
	TotalCarbs  float64
	TopFoods    []FoodCount
	Days        []DayTotal
}

type Store struct {
	db *sql.DB
	tz *time.Location
}

func Open(path string, tz *time.Location) (*Store, error) {
	if tz == nil {
		tz = time.UTC
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	s := &Store{db: db, tz: tz}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS meals (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  items_json TEXT NOT NULL,
  kcal REAL NOT NULL,
  protein REAL NOT NULL,
  fat REAL NOT NULL,
  carbs REAL NOT NULL,
  notes TEXT NOT NULL DEFAULT '',
  confidence TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_meals_user_created ON meals(user_id, created_at);
`)
	return err
}

func (s *Store) AddMeal(userID int64, r *kbju.Result) (int64, error) {
	itemsJSON, err := json.Marshal(r.Items)
	if err != nil {
		return 0, err
	}
	res, err := s.db.Exec(
		`INSERT INTO meals (user_id, created_at, items_json, kcal, protein, fat, carbs, notes, confidence)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userID,
		time.Now().UTC().Format(time.RFC3339Nano),
		string(itemsJSON),
		r.Total.Kcal, r.Total.Protein, r.Total.Fat, r.Total.Carbs,
		r.Notes, r.Confidence,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateMeal(mealID, userID int64, r *kbju.Result) error {
	itemsJSON, err := json.Marshal(r.Items)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(
		`UPDATE meals SET items_json=?, kcal=?, protein=?, fat=?, carbs=?, notes=?, confidence=?
		 WHERE id=? AND user_id=?`,
		string(itemsJSON),
		r.Total.Kcal, r.Total.Protein, r.Total.Fat, r.Total.Carbs,
		r.Notes, r.Confidence,
		mealID, userID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("meal not found")
	}
	return nil
}

func (s *Store) DayTotal(userID int64, day time.Time) (*DayTotal, error) {
	start, end := s.dayBounds(day)
	row := s.db.QueryRow(`
SELECT COUNT(*), COALESCE(SUM(kcal),0), COALESCE(SUM(protein),0), COALESCE(SUM(fat),0), COALESCE(SUM(carbs),0)
FROM meals WHERE user_id=? AND created_at >= ? AND created_at < ?`,
		userID, start.UTC().Format(time.RFC3339Nano), end.UTC().Format(time.RFC3339Nano),
	)
	dt := &DayTotal{Day: day.In(s.tz).Format("2006-01-02")}
	if err := row.Scan(&dt.Meals, &dt.Kcal, &dt.Protein, &dt.Fat, &dt.Carbs); err != nil {
		return nil, err
	}
	return dt, nil
}

func (s *Store) WeekDays(userID int64, now time.Time) ([]DayTotal, error) {
	local := now.In(s.tz)
	startDay := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.tz).AddDate(0, 0, -6)
	var days []DayTotal
	for i := 0; i < 7; i++ {
		d := startDay.AddDate(0, 0, i)
		dt, err := s.DayTotal(userID, d)
		if err != nil {
			return nil, err
		}
		days = append(days, *dt)
	}
	return days, nil
}

func (s *Store) Overview(userID int64, now time.Time, lookbackDays int) (*Overview, error) {
	if lookbackDays <= 0 {
		lookbackDays = 30
	}
	local := now.In(s.tz)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.tz).AddDate(0, 0, -(lookbackDays - 1))

	meals, err := s.mealsSince(userID, start)
	if err != nil {
		return nil, err
	}

	ov := &Overview{}
	byDay := map[string]*DayTotal{}
	foods := map[string]int{}

	for _, m := range meals {
		ov.MealCount++
		ov.TotalKcal += m.Kcal
		ov.TotalProtein += m.Protein
		ov.TotalFat += m.Fat
		ov.TotalCarbs += m.Carbs

		day := m.CreatedAt.In(s.tz).Format("2006-01-02")
		dt, ok := byDay[day]
		if !ok {
			dt = &DayTotal{Day: day}
			byDay[day] = dt
		}
		dt.Meals++
		dt.Kcal += m.Kcal
		dt.Protein += m.Protein
		dt.Fat += m.Fat
		dt.Carbs += m.Carbs

		for _, it := range m.Items {
			name := strings.TrimSpace(strings.ToLower(it.Name))
			if name == "" {
				continue
			}
			foods[name]++
		}
	}

	ov.DayCount = len(byDay)
	if ov.DayCount > 0 {
		n := float64(ov.DayCount)
		ov.AvgKcal = ov.TotalKcal / n
		ov.AvgProtein = ov.TotalProtein / n
		ov.AvgFat = ov.TotalFat / n
		ov.AvgCarbs = ov.TotalCarbs / n
	}

	for day, dt := range byDay {
		_ = day
		ov.Days = append(ov.Days, *dt)
	}
	sortDayTotals(ov.Days)

	ov.TopFoods = topFoods(foods, 8)
	return ov, nil
}

func (s *Store) mealsSince(userID int64, start time.Time) ([]Meal, error) {
	rows, err := s.db.Query(`
SELECT id, user_id, created_at, items_json, kcal, protein, fat, carbs, notes, confidence
FROM meals WHERE user_id=? AND created_at >= ? ORDER BY created_at ASC`,
		userID, start.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Meal
	for rows.Next() {
		var m Meal
		var created, itemsJSON string
		if err := rows.Scan(&m.ID, &m.UserID, &created, &itemsJSON, &m.Kcal, &m.Protein, &m.Fat, &m.Carbs, &m.Notes, &m.Confidence); err != nil {
			return nil, err
		}
		t, err := time.Parse(time.RFC3339Nano, created)
		if err != nil {
			t, err = time.Parse(time.RFC3339, created)
			if err != nil {
				return nil, err
			}
		}
		m.CreatedAt = t
		_ = json.Unmarshal([]byte(itemsJSON), &m.Items)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) dayBounds(day time.Time) (time.Time, time.Time) {
	local := day.In(s.tz)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.tz)
	return start, start.AddDate(0, 0, 1)
}

func sortDayTotals(days []DayTotal) {
	for i := 0; i < len(days); i++ {
		for j := i + 1; j < len(days); j++ {
			if days[j].Day < days[i].Day {
				days[i], days[j] = days[j], days[i]
			}
		}
	}
}

func topFoods(m map[string]int, limit int) []FoodCount {
	var out []FoodCount
	for name, n := range m {
		out = append(out, FoodCount{Name: name, Count: n})
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Count > out[i].Count || (out[j].Count == out[i].Count && out[j].Name < out[i].Name) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
