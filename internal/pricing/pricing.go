// Package pricing turns token counts into an approximate USD cost.
//
// Prices are per 1M tokens and are necessarily a snapshot — providers
// change list prices often. The built-in table is a reasonable default;
// operators can override or extend it by pointing EMBER_PRICING_PATH at a
// JSON file shaped like the Rate map below.
package pricing

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
)

// Rate is the USD cost per 1,000,000 tokens for one model.
type Rate struct {
	InputPerMillion  float64 `json:"inputPerMillion"`
	OutputPerMillion float64 `json:"outputPerMillion"`
}

// defaultRates keys are matched against the reported model string as a
// case-insensitive substring, longest match wins, so "gpt-4o-2024-08-06"
// matches the "gpt-4o" entry without an exact version pin.
var defaultRates = map[string]Rate{
	"gpt-4o-mini":         {InputPerMillion: 0.15, OutputPerMillion: 0.60},
	"gpt-4o":              {InputPerMillion: 2.50, OutputPerMillion: 10.00},
	"gpt-4.1-mini":        {InputPerMillion: 0.40, OutputPerMillion: 1.60},
	"gpt-4.1":             {InputPerMillion: 2.00, OutputPerMillion: 8.00},
	"o1-mini":             {InputPerMillion: 1.10, OutputPerMillion: 4.40},
	"o1":                  {InputPerMillion: 15.00, OutputPerMillion: 60.00},
	"claude-3-5-haiku":    {InputPerMillion: 0.80, OutputPerMillion: 4.00},
	"claude-3-5-sonnet":   {InputPerMillion: 3.00, OutputPerMillion: 15.00},
	"claude-3-opus":       {InputPerMillion: 15.00, OutputPerMillion: 75.00},
	"claude-sonnet-4":     {InputPerMillion: 3.00, OutputPerMillion: 15.00},
	"claude-opus-4":       {InputPerMillion: 15.00, OutputPerMillion: 75.00},
	"gemini-1.5-flash":    {InputPerMillion: 0.075, OutputPerMillion: 0.30},
	"gemini-1.5-pro":      {InputPerMillion: 1.25, OutputPerMillion: 5.00},
	"gemini-2.0-flash":    {InputPerMillion: 0.10, OutputPerMillion: 0.40},
	"llama-3.1-8b":        {InputPerMillion: 0.05, OutputPerMillion: 0.08},
	"llama-3.1-70b":       {InputPerMillion: 0.35, OutputPerMillion: 0.40},
	"mistral-large":       {InputPerMillion: 2.00, OutputPerMillion: 6.00},
}

type Table struct {
	mu    sync.RWMutex
	rates map[string]Rate
}

// Load builds the pricing table from the built-in defaults, optionally
// overlaid with a user-supplied JSON file (EMBER_PRICING_PATH). Entries in
// the override file replace or add to the defaults by key.
func Load(overridePath string) (*Table, error) {
	rates := make(map[string]Rate, len(defaultRates))
	for k, v := range defaultRates {
		rates[k] = v
	}
	if overridePath != "" {
		data, err := os.ReadFile(overridePath)
		if err != nil {
			return nil, err
		}
		var overrides map[string]Rate
		if err := json.Unmarshal(data, &overrides); err != nil {
			return nil, err
		}
		for k, v := range overrides {
			rates[strings.ToLower(k)] = v
		}
	}
	return &Table{rates: rates}, nil
}

// Calculate returns the approximate USD cost for a call to model using
// inputTokens/outputTokens. Unknown models return 0 rather than guessing.
func (t *Table) Calculate(model string, inputTokens, outputTokens int64) float64 {
	if model == "" {
		return 0
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	m := strings.ToLower(model)
	var best Rate
	bestLen := -1
	for key, rate := range t.rates {
		if strings.Contains(m, key) && len(key) > bestLen {
			best = rate
			bestLen = len(key)
		}
	}
	if bestLen < 0 {
		return 0
	}
	return float64(inputTokens)/1_000_000*best.InputPerMillion +
		float64(outputTokens)/1_000_000*best.OutputPerMillion
}
