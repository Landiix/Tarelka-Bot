package bot

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"food-kbju-bot/internal/ai"
	"food-kbju-bot/internal/config"
	"food-kbju-bot/internal/kbju"
	"food-kbju-bot/internal/store"

	tele "gopkg.in/telebot.v3"
)

const (
	btnToday   = "Сегодня"
	btnWeek    = "Неделя"
	btnStats   = "Статистика"
	btnRecs    = "Рекомендации"
	lookbackDays = 30
)

type session struct {
	result *kbju.Result
	mealID int64
}

type Bot struct {
	tb     *tele.Bot
	cfg    *config.Config
	ai     *ai.Client
	store  *store.Store
	mu     sync.Mutex
	last   map[int64]*session
	logger *slog.Logger
	menu   *tele.ReplyMarkup
}

func New(cfg *config.Config, aiClient *ai.Client, st *store.Store, logger *slog.Logger) (*Bot, error) {
	tb, err := tele.NewBot(tele.Settings{
		Token:  cfg.BotToken,
		Poller: &tele.LongPoller{Timeout: 20 * time.Second},
	})
	if err != nil {
		return nil, fmt.Errorf("telebot: %w", err)
	}

	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	menu.Reply(
		menu.Row(menu.Text(btnToday), menu.Text(btnWeek)),
		menu.Row(menu.Text(btnStats), menu.Text(btnRecs)),
	)

	b := &Bot{
		tb:     tb,
		cfg:    cfg,
		ai:     aiClient,
		store:  st,
		last:   make(map[int64]*session),
		logger: logger,
		menu:   menu,
	}
	b.register()
	return b, nil
}

func (b *Bot) Start() {
	b.logger.Info("bot started", "username", b.tb.Me.Username)
	b.tb.Start()
}

func (b *Bot) Stop() {
	b.tb.Stop()
}

func (b *Bot) register() {
	b.tb.Use(b.allowlistMiddleware)

	b.tb.Handle("/start", b.handleStart)
	b.tb.Handle("/today", b.handleToday)
	b.tb.Handle("/week", b.handleWeek)
	b.tb.Handle("/stats", b.handleStats)
	b.tb.Handle("/recs", b.handleRecs)
	b.tb.Handle(tele.OnPhoto, b.handlePhoto)
	b.tb.Handle(tele.OnText, b.handleText)
}

func (b *Bot) allowlistMiddleware(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		user := c.Sender()
		if user == nil {
			return nil
		}
		if !b.cfg.IsAllowed(user.ID) {
			return c.Send("Доступ закрыт. Напиши владельцу бота, чтобы добавил твой Telegram ID.")
		}
		return next(c)
	}
}

func (b *Bot) handleStart(c tele.Context) error {
	text := `Привет! Пришли фото еды — оценю КБЖУ и сохраню в дневник.

Это приблизительная оценка по фото, не лабораторный анализ.

Как пользоваться:
1) Отправь фото тарелки (можно с подписью)
2) Уточни текстом: «это 300 г пасты»
3) Кнопки ниже: день, неделя, статистика, рекомендации`

	return c.Send(text, b.menu)
}

func (b *Bot) handlePhoto(c tele.Context) error {
	msg := c.Message()
	if msg == nil || msg.Photo == nil {
		return c.Send("Не вижу фото. Пришли картинку еды.", b.menu)
	}

	_ = c.Notify(tele.Typing)

	photo := msg.Photo
	reader, err := b.tb.File(&photo.File)
	if err != nil {
		b.logger.Error("download photo", "err", err)
		return c.Send("Не удалось скачать фото. Попробуй ещё раз.", b.menu)
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, 15<<20))
	if err != nil {
		b.logger.Error("read photo", "err", err)
		return c.Send("Не удалось прочитать фото.", b.menu)
	}

	mime := http.DetectContentType(data)
	if mime == "application/octet-stream" {
		mime = "image/jpeg"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	result, err := b.ai.AnalyzePhoto(ctx, data, mime, msg.Caption)
	if err != nil {
		b.logger.Error("analyze photo", "err", err, "user", c.Sender().ID)
		return c.Send(userFacingAIError(err), b.menu)
	}

	mealID, err := b.store.AddMeal(c.Sender().ID, result)
	if err != nil {
		b.logger.Error("save meal", "err", err, "user", c.Sender().ID)
		return c.Send(kbju.FormatReply(result)+"\n\n⚠️ Не удалось сохранить в дневник.", b.menu)
	}

	b.setLast(c.Sender().ID, result, mealID)

	reply := kbju.FormatReply(result) + "\n\n✅ Сохранено в дневник."
	if day, err := b.store.DayTotal(c.Sender().ID, time.Now()); err == nil && day.Meals > 0 {
		reply += fmt.Sprintf("\nСегодня суммарно: К %.0f / Б %.0f / Ж %.0f / У %.0f", day.Kcal, day.Protein, day.Fat, day.Carbs)
	}
	return c.Send(reply, b.menu)
}

func (b *Bot) handleText(c tele.Context) error {
	text := strings.TrimSpace(c.Text())
	if text == "" || text == "/start" {
		return nil
	}

	switch text {
	case btnToday, "/today":
		return b.handleToday(c)
	case btnWeek, "/week":
		return b.handleWeek(c)
	case btnStats, "/stats":
		return b.handleStats(c)
	case btnRecs, "/recs":
		return b.handleRecs(c)
	}

	prev := b.getLast(c.Sender().ID)
	if prev == nil || prev.result == nil {
		return c.Send("Сначала пришли фото еды — потом можно уточнять порцию текстом.\nИли жми кнопки статистики.", b.menu)
	}

	_ = c.Notify(tele.Typing)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := b.ai.Refine(ctx, prev.result, text)
	if err != nil {
		b.logger.Error("refine", "err", err, "user", c.Sender().ID)
		return c.Send(userFacingAIError(err), b.menu)
	}

	if prev.mealID > 0 {
		if err := b.store.UpdateMeal(prev.mealID, c.Sender().ID, result); err != nil {
			b.logger.Error("update meal", "err", err, "user", c.Sender().ID, "meal", prev.mealID)
		}
	}

	b.setLast(c.Sender().ID, result, prev.mealID)
	return c.Send(kbju.FormatReply(result)+"\n\n✅ Дневник обновлён.", b.menu)
}

func (b *Bot) handleToday(c tele.Context) error {
	dt, err := b.store.DayTotal(c.Sender().ID, time.Now())
	if err != nil {
		b.logger.Error("day total", "err", err)
		return c.Send("Не удалось посчитать день.", b.menu)
	}
	return c.Send(formatDayTotal(dt), b.menu)
}

func (b *Bot) handleWeek(c tele.Context) error {
	days, err := b.store.WeekDays(c.Sender().ID, time.Now())
	if err != nil {
		b.logger.Error("week stats", "err", err)
		return c.Send("Не удалось посчитать неделю.", b.menu)
	}
	return c.Send(formatWeekStats(days), b.menu)
}

func (b *Bot) handleStats(c tele.Context) error {
	ov, err := b.store.Overview(c.Sender().ID, time.Now(), lookbackDays)
	if err != nil {
		b.logger.Error("overview", "err", err)
		return c.Send("Не удалось собрать статистику.", b.menu)
	}
	return c.Send(formatOverview(ov, lookbackDays), b.menu)
}

func (b *Bot) handleRecs(c tele.Context) error {
	ov, err := b.store.Overview(c.Sender().ID, time.Now(), lookbackDays)
	if err != nil {
		b.logger.Error("overview for recs", "err", err)
		return c.Send("Не удалось собрать статистику для рекомендаций.", b.menu)
	}
	if ov.MealCount == 0 {
		return c.Send("Пока мало данных для рекомендаций. Пришли несколько фото еды за день-два.", b.menu)
	}

	_ = c.Notify(tele.Typing)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	text, err := b.ai.Recommend(ctx, overviewPrompt(ov, lookbackDays))
	if err != nil {
		b.logger.Error("recommend", "err", err, "user", c.Sender().ID)
		return c.Send(userFacingAIError(err), b.menu)
	}
	return c.Send("Рекомендации по твоей статистике:\n\n"+text, b.menu)
}

func userFacingAIError(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "404"), strings.Contains(msg, "not found"), strings.Contains(msg, "no longer available"):
		return "Модель Gemini недоступна. Проверь GEMINI_MODEL в .env (сейчас лучше gemini-3.1-flash-lite) и перезапусти бота."
	case strings.Contains(msg, "401"), strings.Contains(msg, "403"), strings.Contains(msg, "api key"), strings.Contains(msg, "permission"):
		return "Проблема с GEMINI_API_KEY. Проверь ключ в .env и перезапусти бота."
	case strings.Contains(msg, "503"), strings.Contains(msg, "unavailable"), strings.Contains(msg, "high demand"), strings.Contains(msg, "429"):
		return "Gemini сейчас перегружен. Подожди минуту и пришли фото ещё раз."
	default:
		return "Не получилось разобрать фото. Попробуй другое или уточни подписью."
	}
}

func (b *Bot) setLast(userID int64, r *kbju.Result, mealID int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.last[userID] = &session{result: r, mealID: mealID}
}

func (b *Bot) getLast(userID int64) *session {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.last[userID]
}
