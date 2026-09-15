package aigateway

import "math"

type TokenUsage struct {
	InputTokens           int `json:"input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	CacheCreationTokens   int `json:"cache_creation_tokens,omitempty"`
	CacheReadTokens       int `json:"cache_read_tokens,omitempty"`
	CacheCreation5mTokens int `json:"cache_creation_5m_tokens,omitempty"`
	CacheCreation1hTokens int `json:"cache_creation_1h_tokens,omitempty"`
	ImageOutputTokens     int `json:"image_output_tokens,omitempty"`
}

// Providers report cumulative counters, sometimes in separate SSE events.
func mergeTokenUsage(current, update TokenUsage) TokenUsage {
	return TokenUsage{
		InputTokens:           max(current.InputTokens, update.InputTokens),
		OutputTokens:          max(current.OutputTokens, update.OutputTokens),
		CacheCreationTokens:   max(current.CacheCreationTokens, update.CacheCreationTokens),
		CacheReadTokens:       max(current.CacheReadTokens, update.CacheReadTokens),
		CacheCreation5mTokens: max(current.CacheCreation5mTokens, update.CacheCreation5mTokens),
		CacheCreation1hTokens: max(current.CacheCreation1hTokens, update.CacheCreation1hTokens),
		ImageOutputTokens:     max(current.ImageOutputTokens, update.ImageOutputTokens),
	}
}

type Price struct {
	InputPer1K                  float64 `json:"input_price_per_1k"`
	OutputPer1K                 float64 `json:"output_price_per_1k"`
	CacheCreationPer1K          float64 `json:"cache_creation_price_per_1k,omitempty"`
	CacheReadPer1K              float64 `json:"cache_read_price_per_1k,omitempty"`
	CacheCreation5mPer1K        float64 `json:"cache_creation_5m_price_per_1k,omitempty"`
	CacheCreation1hPer1K        float64 `json:"cache_creation_1h_price_per_1k,omitempty"`
	ImageOutputPer1K            float64 `json:"image_output_price_per_1k,omitempty"`
	PriorityInputPer1K          float64 `json:"priority_input_price_per_1k,omitempty"`
	PriorityOutputPer1K         float64 `json:"priority_output_price_per_1k,omitempty"`
	PriorityCacheReadPer1K      float64 `json:"priority_cache_read_price_per_1k,omitempty"`
	LongContextThreshold        int     `json:"long_context_threshold,omitempty"`
	LongContextInputMultiplier  float64 `json:"long_context_input_multiplier,omitempty"`
	LongContextOutputMultiplier float64 `json:"long_context_output_multiplier,omitempty"`
	BillingMode                 string  `json:"billing_mode,omitempty"`
	PricingSource               string  `json:"pricing_source,omitempty"`
}

type CostBreakdown struct {
	InputCost         float64 `json:"input_cost"`
	OutputCost        float64 `json:"output_cost"`
	CacheCreationCost float64 `json:"cache_creation_cost"`
	CacheReadCost     float64 `json:"cache_read_cost"`
	ImageOutputCost   float64 `json:"image_output_cost"`
	TotalCost         float64 `json:"total_cost"`
	ActualCost        float64 `json:"actual_cost"`
	RateMultiplier    float64 `json:"rate_multiplier"`
	BillingMode       string  `json:"billing_mode"`
	ServiceTier       string  `json:"service_tier,omitempty"`
}

func CalculateCost(usage TokenUsage, price Price) float64 {
	return CalculateCostBreakdown(usage, price, 1, "").ActualCost
}

func CalculateCostBreakdown(usage TokenUsage, price Price, rateMultiplier float64, serviceTier string) CostBreakdown {
	if rateMultiplier < 0 {
		rateMultiplier = 0
	}
	inputPrice := price.InputPer1K
	outputPrice := price.OutputPer1K
	cacheReadPrice := price.CacheReadPer1K
	tierMultiplier := serviceTierMultiplier(serviceTier)
	if serviceTier == "priority" {
		if price.PriorityInputPer1K > 0 {
			inputPrice = price.PriorityInputPer1K
			tierMultiplier = 1
		}
		if price.PriorityOutputPer1K > 0 {
			outputPrice = price.PriorityOutputPer1K
			tierMultiplier = 1
		}
		if price.PriorityCacheReadPer1K > 0 {
			cacheReadPrice = price.PriorityCacheReadPer1K
			tierMultiplier = 1
		}
	}
	if shouldApplyLongContext(usage, price) {
		inputPrice *= nonZeroMultiplier(price.LongContextInputMultiplier)
		outputPrice *= nonZeroMultiplier(price.LongContextOutputMultiplier)
	}
	textOutputTokens := usage.OutputTokens - usage.ImageOutputTokens
	if textOutputTokens < 0 {
		textOutputTokens = 0
	}
	cacheCreationCost := price.CacheCreationPer1K * float64(usage.CacheCreationTokens) / 1000
	if price.CacheCreation5mPer1K > 0 || price.CacheCreation1hPer1K > 0 {
		cacheCreationCost = (price.CacheCreation5mPer1K*float64(usage.CacheCreation5mTokens) + price.CacheCreation1hPer1K*float64(usage.CacheCreation1hTokens)) / 1000
	}
	imagePrice := price.ImageOutputPer1K
	if imagePrice == 0 {
		imagePrice = outputPrice
	}
	breakdown := CostBreakdown{
		InputCost:         roundCost(inputPrice * float64(usage.InputTokens) / 1000 * tierMultiplier),
		OutputCost:        roundCost(outputPrice * float64(textOutputTokens) / 1000 * tierMultiplier),
		CacheCreationCost: roundCost(cacheCreationCost * tierMultiplier),
		CacheReadCost:     roundCost(cacheReadPrice * float64(usage.CacheReadTokens) / 1000 * tierMultiplier),
		ImageOutputCost:   roundCost(imagePrice * float64(usage.ImageOutputTokens) / 1000 * tierMultiplier),
		RateMultiplier:    rateMultiplier,
		BillingMode:       price.BillingMode,
		ServiceTier:       serviceTier,
	}
	if breakdown.BillingMode == "" {
		breakdown.BillingMode = "token"
	}
	breakdown.TotalCost = roundCost(breakdown.InputCost + breakdown.OutputCost + breakdown.CacheCreationCost + breakdown.CacheReadCost + breakdown.ImageOutputCost)
	breakdown.ActualCost = roundCost(breakdown.TotalCost * rateMultiplier)
	return breakdown
}

func roundCost(value float64) float64 {
	return math.Round(value*1e8) / 1e8
}

func serviceTierMultiplier(serviceTier string) float64 {
	switch serviceTier {
	case "priority":
		return 2
	case "flex":
		return 0.5
	default:
		return 1
	}
}

func nonZeroMultiplier(value float64) float64 {
	if value == 0 {
		return 1
	}
	return value
}

func shouldApplyLongContext(usage TokenUsage, price Price) bool {
	return price.LongContextThreshold > 0 && usage.InputTokens+usage.CacheReadTokens >= price.LongContextThreshold
}
