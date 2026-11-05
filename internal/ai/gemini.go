package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"food-kbju-bot/internal/kbju"

	"google.golang.org/genai"
)

const systemPrompt = `Ты оцениваешь КБЖУ еды по фото для личного трекинга.
Правила:
1) Распознай блюда на фото.
2) Оцени порцию в граммах на глаз.
3) Посчитай ккал, белки, жиры, углеводы на всю порцию каждого блюда и итого.
4) Учти очевидные допущения (масло, соус, если вероятно).
5) Отвечай ТОЛЬКО валидным JSON без markdown по схеме:
{
  "items": [{"name":"string","grams":number,"kcal":number,"protein":number,"fat":number,"carbs":number}],
  "total": {"kcal":number,"protein":number,"fat":number,"carbs":number},
  "confidence": "high"|"medium"|"low",
  "notes": "string"
}
Числа — реалистичные оценки. Язык name/notes — русский.`

type Client struct {
	genai *genai.Client
	model string
}

func New(ctx context.Context, apiKey, model string) (*Client, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("genai client: %w", err)
	}
	return &Client{genai: client, model: model}, nil
}

func (c *Client) AnalyzePhoto(ctx context.Context, image []byte, mimeType, caption string) (*kbju.Result, error) {
	userText := "Оцени КБЖУ еды на фото."
	if strings.TrimSpace(caption) != "" {
		userText += "\nУточнение пользователя: " + strings.TrimSpace(caption)
	}

	parts := []*genai.Part{
		{Text: userText},
		{InlineData: &genai.Blob{Data: image, MIMEType: mimeType}},
	}

	return c.generate(ctx, parts)
}

func (c *Client) Refine(ctx context.Context, previous *kbju.Result, clarification string) (*kbju.Result, error) {
	prevJSON, err := json.Marshal(previous)
	if err != nil {
		return nil, fmt.Errorf("marshal previous: %w", err)
	}

	text := fmt.Sprintf(
		"Предыдущая оценка КБЖУ (JSON):\n%s\n\nУточнение пользователя: %s\n\nПересчитай КБЖУ с учётом уточнения. Верни JSON той же схемы.",
		string(prevJSON),
		strings.TrimSpace(clarification),
	)

	return c.generate(ctx, []*genai.Part{{Text: text}})
}

func (c *Client) Recommend(ctx context.Context, statsSummary string) (string, error) {
	prompt := `Ты нутрициолог-консультант для обычного человека.
На основе статистики питания дай практичные рекомендации на русском.
Формат:
1) Короткий вывод (1-2 предложения)
2) 4-6 конкретных советов списком
3) На что обратить внимание завтра/на неделе
Без медицинских диагнозов. Не выдумывай цифры сверх данных. Оценка приблизительная.
Статистика пользователя:
` + statsSummary

	cfg := &genai.GenerateContentConfig{
		Temperature: genai.Ptr(float32(0.4)),
	}
	contents := []*genai.Content{
		{Role: genai.RoleUser, Parts: []*genai.Part{{Text: prompt}}},
	}

	var (
		resp *genai.GenerateContentResponse
		err  error
	)
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err = c.genai.Models.GenerateContent(ctx, c.model, contents, cfg)
		if err == nil {
			break
		}
		if !isRetryable(err) || attempt == 3 {
			return "", fmt.Errorf("recommend (model=%s): %w", c.model, err)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}

	text := strings.TrimSpace(resp.Text())
	if text == "" {
		return "", fmt.Errorf("empty recommendations")
	}
	return text, nil
}

func (c *Client) generate(ctx context.Context, parts []*genai.Part) (*kbju.Result, error) {
	cfg := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemPrompt}},
		},
		ResponseMIMEType: "application/json",
		ResponseSchema:   responseSchema(),
		Temperature:      genai.Ptr(float32(0.2)),
	}

	contents := []*genai.Content{
		{Role: genai.RoleUser, Parts: parts},
	}

	var (
		resp *genai.GenerateContentResponse
		err  error
	)
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err = c.genai.Models.GenerateContent(ctx, c.model, contents, cfg)
		if err == nil {
			break
		}
		if !isRetryable(err) || attempt == 3 {
			return nil, fmt.Errorf("generate content (model=%s): %w", c.model, err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}

	raw := strings.TrimSpace(resp.Text())
	if raw == "" {
		return nil, fmt.Errorf("empty model response")
	}

	var result kbju.Result
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("parse json: %w; raw=%s", err, truncate(raw, 400))
	}
	return &result, nil
}

func isRetryable(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "503") ||
		strings.Contains(msg, "unavailable") ||
		strings.Contains(msg, "high demand") ||
		strings.Contains(msg, "429") ||
		strings.Contains(msg, "resource_exhausted")
}

func responseSchema() *genai.Schema {
	nutrients := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"kcal":    {Type: genai.TypeNumber},
			"protein": {Type: genai.TypeNumber},
			"fat":     {Type: genai.TypeNumber},
			"carbs":   {Type: genai.TypeNumber},
		},
		Required: []string{"kcal", "protein", "fat", "carbs"},
	}

	item := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"name":    {Type: genai.TypeString},
			"grams":   {Type: genai.TypeNumber},
			"kcal":    {Type: genai.TypeNumber},
			"protein": {Type: genai.TypeNumber},
			"fat":     {Type: genai.TypeNumber},
			"carbs":   {Type: genai.TypeNumber},
		},
		Required: []string{"name", "grams", "kcal", "protein", "fat", "carbs"},
	}

	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"items": {
				Type:  genai.TypeArray,
				Items: item,
			},
			"total":      nutrients,
			"confidence": {Type: genai.TypeString},
			"notes":      {Type: genai.TypeString},
		},
		Required: []string{"items", "total", "confidence", "notes"},
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
