package mllm

type LMessage struct {
	Choices           []LChoices `json:"choices"`
	Created           int        `json:"created"`
	ID                string     `json:"id"`
	Model             string     `json:"model"`
	SystemFingerprint string     `json:"system_fingerprint"`
	Object            string     `json:"object"`
	Timings           LTimings   `json:"timings"`
}
type (
	LDelta   struct{}
	LChoices struct {
		FinishReason string `json:"finish_reason"`
		Index        int    `json:"index"`
		Delta        LDelta `json:"delta"`
	}
)

type LTimings struct {
	CacheN              int     `json:"cache_n"`
	PromptN             int     `json:"prompt_n"`
	PromptMs            float64 `json:"prompt_ms"`
	PromptPerTokenMs    float64 `json:"prompt_per_token_ms"`
	PromptPerSecond     float64 `json:"prompt_per_second"`
	PredictedN          int     `json:"predicted_n"`
	PredictedMs         float64 `json:"predicted_ms"`
	PredictedPerTokenMs float64 `json:"predicted_per_token_ms"`
	PredictedPerSecond  float64 `json:"predicted_per_second"`
}
