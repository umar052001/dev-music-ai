package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ─── LLM Provider Abstraction ───────────────────────────────────────────────
//
// Supported providers: ollama, openai, groq, anthropic (claude), gemini
// Configuration comes from config.json and can be overridden via the UI/API.

type LLMProvider string

const (
	ProviderOllama    LLMProvider = "ollama"
	ProviderOpenAI    LLMProvider = "openai"
	ProviderGroq      LLMProvider = "groq"
	ProviderAnthropic LLMProvider = "anthropic"
	ProviderGemini    LLMProvider = "gemini"
)

// NeedsAPIKey reports whether a provider requires an API key to function.
func (p LLMProvider) NeedsAPIKey() bool {
	switch p {
	case ProviderOllama:
		return false
	default:
		return true
	}
}

// ProviderDisplayName returns a friendly UI label.
func (p LLMProvider) DisplayName() string {
	switch p {
	case ProviderOllama:
		return "Ollama"
	case ProviderOpenAI:
		return "OpenAI"
	case ProviderGroq:
		return "Groq"
	case ProviderAnthropic:
		return "Anthropic (Claude)"
	case ProviderGemini:
		return "Google Gemini"
	}
	return string(p)
}

type LLMConfig struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`       // smart/primary model
	FastModel  string `json:"fast_model"`  // cheap model for simple tasks
	APIKey     string `json:"api_key"`
	APIBase    string `json:"api_base"`    // custom base URL (ollama endpoint, openai-compatible proxies)
	TimeoutSec int    `json:"timeout_sec"`

	// Per-provider model defaults
	OllamaLocalModel  string `json:"ollama_local_model"`
	OllamaCloudModel  string `json:"ollama_cloud_model"`
	OpenAIModel       string `json:"openai_model"`
	GroqModel         string `json:"groq_model"`
	AnthropicModel    string `json:"anthropic_model"`
	GeminiModel       string `json:"gemini_model"`
}

func DefaultLLMConfig() LLMConfig {
	return LLMConfig{
		Provider:         "ollama",
		Model:            "kimi-k2.6:cloud",
		FastModel:        "gemma4:cloud",
		APIBase:          "http://localhost:11434",
		TimeoutSec:       300,
		OllamaLocalModel: "gemma4:latest",
		OllamaCloudModel: "kimi-k2.6:cloud",
		OpenAIModel:      "gpt-4o-mini",
		GroqModel:        "llama-3.3-70b-versatile",
		AnthropicModel:   "claude-sonnet-4-5",
		GeminiModel:      "gemini-2.0-flash",
	}
}

var (
	llmMu     sync.RWMutex
	llmConfig = DefaultLLMConfig()
	llmConfigPath string
)

func initLLM() {
	// Determine config path: default alongside binary or CWD.
	exe, err := os.Executable()
	if err != nil {
		exe = "."
	}
	llmConfigPath = filepath.Join(filepath.Dir(exe), "config.json")
	// Allow override via env var
	if p := os.Getenv("DEVMUSIC_CONFIG"); p != "" {
		llmConfigPath = p
	}
	loadLLMConfig()
}

func loadLLMConfig() {
	data, err := os.ReadFile(llmConfigPath)
	if err != nil {
		return // use defaults
	}
	var c LLMConfig
	if err := json.Unmarshal(data, &c); err != nil {
		log.Printf("llm: could not parse config: %v", err)
		return
	}
	llmMu.Lock()
	// Merge over defaults so missing fields keep sensible values
	d := DefaultLLMConfig()
	if c.Provider != "" {
		d.Provider = c.Provider
	}
	if c.Model != "" {
		d.Model = c.Model
	}
	if c.FastModel != "" {
		d.FastModel = c.FastModel
	}
	if c.APIKey != "" {
		d.APIKey = c.APIKey
	}
	if c.APIBase != "" {
		d.APIBase = c.APIBase
	}
	if c.TimeoutSec > 0 {
		d.TimeoutSec = c.TimeoutSec
	}
	if c.OllamaLocalModel != "" {
		d.OllamaLocalModel = c.OllamaLocalModel
	}
	if c.OllamaCloudModel != "" {
		d.OllamaCloudModel = c.OllamaCloudModel
	}
	if c.OpenAIModel != "" {
		d.OpenAIModel = c.OpenAIModel
	}
	if c.GroqModel != "" {
		d.GroqModel = c.GroqModel
	}
	if c.AnthropicModel != "" {
		d.AnthropicModel = c.AnthropicModel
	}
	if c.GeminiModel != "" {
		d.GeminiModel = c.GeminiModel
	}
	llmConfig = d
	llmMu.Unlock()
	// Ensure default base URL for common providers if empty
	llmMu.Lock()
	applyProviderDefaultsLocked()
	llmMu.Unlock()
}

func applyProviderDefaultsLocked() {
	// If APIBase empty, set the well-known default for the selected provider.
	if llmConfig.APIBase == "" {
		switch LLMProvider(llmConfig.Provider) {
		case ProviderOllama:
			llmConfig.APIBase = "http://localhost:11434"
		case ProviderOpenAI:
			llmConfig.APIBase = "https://api.openai.com/v1"
		case ProviderGroq:
			llmConfig.APIBase = "https://api.groq.com/openai/v1"
		case ProviderGemini:
			llmConfig.APIBase = "https://generativelanguage.googleapis.com/v1beta"
		}
	}
	// Resolve model name per provider if not explicitly set
	p := LLMProvider(llmConfig.Provider)
	if llmConfig.Model == "" {
		switch p {
		case ProviderOllama:
			llmConfig.Model = llmConfig.OllamaCloudModel
		case ProviderOpenAI:
			llmConfig.Model = llmConfig.OpenAIModel
		case ProviderGroq:
			llmConfig.Model = llmConfig.GroqModel
		case ProviderAnthropic:
			llmConfig.Model = llmConfig.AnthropicModel
		case ProviderGemini:
			llmConfig.Model = llmConfig.GeminiModel
		}
	}
	if llmConfig.FastModel == "" {
		switch p {
		case ProviderOllama:
			llmConfig.FastModel = llmConfig.OllamaLocalModel
		case ProviderOpenAI:
			llmConfig.FastModel = llmConfig.OpenAIModel
		case ProviderGroq:
			llmConfig.FastModel = llmConfig.GroqModel
		case ProviderAnthropic:
			llmConfig.FastModel = llmConfig.AnthropicModel
		case ProviderGemini:
			llmConfig.FastModel = llmConfig.GeminiModel
		}
	}
}

func saveLLMConfig() error {
	llmMu.Lock()
	defer llmMu.Unlock()
	data, err := json.MarshalIndent(llmConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(llmConfigPath, data, 0644)
}

// getLLMConfig returns a copy of the current config (safe to read).
func getLLMConfig() LLMConfig {
	llmMu.RLock()
	defer llmMu.RUnlock()
	return llmConfig
}

// setLLMConfig atomically updates config and persists it.
func setLLMConfig(c LLMConfig) error {
	llmMu.Lock()
	llmConfig = c
	applyProviderDefaultsLocked()
	llmMu.Unlock()
	return saveLLMConfig()
}

// modelFor selects the model to use: prefer "smart", but allow an explicit
// "fast" override for cheap tasks.
func modelFor(fast bool) (string, LLMProvider, string) {
	llmMu.RLock()
	c := llmConfig
	llmMu.RUnlock()
	m := c.Model
	if fast && c.FastModel != "" {
		m = c.FastModel
	}
	return m, LLMProvider(c.Provider), c.APIKey
}

// providerHealth briefly checks whether the selected provider is reachable,
// and whether a fallback (local Ollama) would be available.
type LLMHealth struct {
	Provider    string   `json:"provider"`
	ProviderOK  bool     `json:"provider_ok"`
	Model       string   `json:"model"`
	APIKeySet   bool     `json:"api_key_set"`
	Available   bool     `json:"available"`   // at least one provider is usable
	Cloud       bool     `json:"cloud"`       // selected provider is capable (not local-only)
	LocalOllamaOK bool   `json:"local_ollama_ok"`
	Error       string   `json:"error,omitempty"`
}

func checkLLMHealth() LLMHealth {
	llmMu.RLock()
	c := llmConfig
	llmMu.RUnlock()

	p := LLMProvider(c.Provider)
	h := LLMHealth{
		Provider:  string(p),
		Model:     c.Model,
		APIKeySet: c.APIKey != "",
		Cloud:     true,
	}

	// Determine cloud-ness: Ollama itself is ambiguous (can be local or cloud).
	// If the user points ollama at a localhost server, treat it as not-cloud
	// unless a "cloud" model is chosen. Other providers are inherently cloud.
	if p == ProviderOllama {
		h.Cloud = !strings.Contains(strings.ToLower(c.APIBase), "localhost") ||
			strings.Contains(strings.ToLower(c.Model), "cloud")
	}

	switch p {
	case ProviderOllama:
		ok, err := ollamaPing(c.APIBase)
		h.ProviderOK = ok
		if err != nil {
			h.Error = err.Error()
		}
	case ProviderOpenAI, ProviderGroq:
		h.ProviderOK = c.APIKey != ""
		if c.APIKey == "" {
			h.Error = "no API key configured"
		}
	case ProviderAnthropic:
		h.ProviderOK = c.APIKey != ""
		if c.APIKey == "" {
			h.Error = "no API key configured"
		}
	case ProviderGemini:
		h.ProviderOK = c.APIKey != ""
		if c.APIKey == "" {
			h.Error = "no API key configured"
		}
	default:
		h.ProviderOK = false
		h.Error = "unknown provider"
	}

	// Determine whether any usable provider exists as a fallback.
	localOllamaOK, _ := ollamaPing("http://localhost:11434")
	h.LocalOllamaOK = localOllamaOK

	h.Available = h.ProviderOK || localOllamaOK
	return h
}

func ollamaPing(base string) (bool, error) {
	base = strings.TrimRight(base, "/")
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(base + "/api/tags")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200, nil
}

// AskLLM is the unified entry point. It tries the configured provider, and on
// failure falls back to a local Ollama server if one is reachable.
// AskLLM sends a chat request to the configured (or fallback) provider and
// returns the text response. maxTokens (optional) caps the generated output;
// when omitted, each adapter's default is used. Use a large value for tasks
// that must emit long output (e.g. batch parsing long lists).
func AskLLM(system, user string, temperature float64, fast bool, maxTokens ...int) (string, error) {
	mt := 0
	if len(maxTokens) > 0 {
		mt = maxTokens[0]
	}
	model, provider, apiKey := modelFor(fast)
	out, err := askProvider(provider, model, system, user, temperature, apiKey, mt)
	if err == nil {
		return out, nil
	}

	// Fallback: try local Ollama
	log.Printf("llm: provider %s error (%v); trying local ollama fallback", provider, err)
	fallbackModel := "gemma4:latest"
	// Check if local ollama has a preferred local model
	if l := getLLMConfig(); l.OllamaLocalModel != "" {
		fallbackModel = l.OllamaLocalModel
	}
	out2, err2 := askOllama("http://localhost:11434", fallbackModel, system, user, temperature, mt)
	if err2 == nil {
		return out2, nil
	}
	return "", fmt.Errorf("all LLM providers failed: %v; fallback: %v", err, err2)
}

// askProvider dispatches to the correct provider implementation.
func askProvider(provider LLMProvider, model, system, user string, temperature float64, apiKey string, maxTokens int) (string, error) {
	switch provider {
	case ProviderOllama:
		base := getLLMConfig().APIBase
		return askOllama(base, model, system, user, temperature, maxTokens)
	case ProviderOpenAI, ProviderGroq:
		base := getLLMConfig().APIBase
		return askOpenAICompat(base, model, system, user, temperature, apiKey, maxTokens)
	case ProviderAnthropic:
		return askAnthropic(model, system, user, temperature, apiKey, maxTokens)
	case ProviderGemini:
		base := getLLMConfig().APIBase
		return askGemini(base, model, system, user, temperature, apiKey, maxTokens)
	default:
		return "", fmt.Errorf("unknown provider: %s", provider)
	}
}

// ─── Ollama ─────────────────────────────────────────────────────────────────

func askOllama(base, model, system, user string, temperature float64, maxTokens int) (string, error) {
	client := &http.Client{Timeout: time.Duration(llmTimeoutSec()) * time.Second}
	prompt := system
	if user != "" {
		prompt = system + "\n\n" + user
	}
	numPredict := 2048
	if maxTokens > 0 {
		numPredict = maxTokens
	}
	body := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": temperature,
			"num_predict": numPredict,
		},
	}
	jsonBody, _ := json.Marshal(body)
	base = strings.TrimRight(base, "/")
	resp, err := client.Post(base+"/api/generate", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama %s: %s", resp.Status, string(b))
	}
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if r, ok := result["response"].(string); ok {
		return r, nil
	}
	return "", fmt.Errorf("ollama: no response")
}

func llmTimeoutSec() int {
	llmMu.RLock()
	defer llmMu.RUnlock()
	if llmConfig.TimeoutSec > 0 {
		return llmConfig.TimeoutSec
	}
	return 120
}

// ─── OpenAI-compatible (OpenAI, Groq, OpenRouter etc.) ──────────────────────

func askOpenAICompat(base, model, system, user string, temperature float64, apiKey string, maxTokens int) (string, error) {
	client := &http.Client{Timeout: time.Duration(llmTimeoutSec()) * time.Second}
	base = strings.TrimRight(base, "/")
	body := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": temperature,
	}
	if maxTokens > 0 {
		body["max_tokens"] = maxTokens
	}
	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, base+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai-compat %s: %s", resp.Status, string(b))
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("openai-compat: no choices")
}

// ─── Anthropic (Claude) ─────────────────────────────────────────────────────

func askAnthropic(model, system, user string, temperature float64, apiKey string, maxTokens int) (string, error) {
	client := &http.Client{Timeout: time.Duration(llmTimeoutSec()) * time.Second}
	mt := 4096
	if maxTokens > 0 {
		mt = maxTokens
	}
	body := map[string]interface{}{
		"model":      model,
		"max_tokens": mt,
		"system":     system,
		"messages": []map[string]string{
			{"role": "user", "content": user},
		},
	}
	// Anthropic doesn't accept temperature of 0 on some models; clamp.
	if temperature == 0 {
		temperature = 0.1
	}
	body["temperature"] = temperature
	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("anthropic %s: %s", resp.Status, string(b))
	}
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, c := range result.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	if sb.Len() == 0 {
		return "", fmt.Errorf("anthropic: empty response")
	}
	return sb.String(), nil
}

// ─── Google Gemini ──────────────────────────────────────────────────────────

func askGemini(base, model, system, user string, temperature float64, apiKey string, maxTokens int) (string, error) {
	client := &http.Client{Timeout: time.Duration(llmTimeoutSec()) * time.Second}
	body := map[string]interface{}{
		"systemInstruction": map[string]interface{}{
			"parts": []map[string]string{{"text": system}},
		},
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]string{{"text": user}}},
		},
		"generationConfig": map[string]interface{}{
			"temperature": temperature,
		},
	}
	if maxTokens > 0 {
		body["generationConfig"].(map[string]interface{})["maxOutputTokens"] = maxTokens
	}
	jsonBody, _ := json.Marshal(body)
	base = strings.TrimRight(base, "/")
	url := base + "/models/" + model + ":generateContent?key=" + apiKey
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gemini %s: %s", resp.Status, string(b))
	}
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		return result.Candidates[0].Content.Parts[0].Text, nil
	}
	return "", fmt.Errorf("gemini: empty response")
}

// ─── HTTP handlers ──────────────────────────────────────────────────────────

func handleLLMStatus(w http.ResponseWriter, r *http.Request) {
	h := checkLLMHealth()
	json.NewEncoder(w).Encode(h)
}

func handleLLMConfigGet(w http.ResponseWriter, r *http.Request) {
	c := getLLMConfig()
	// Never leak the API key back to the client fully; send a masked version.
	key := ""
	if c.APIKey != "" {
		key = "(set)"
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"provider":          c.Provider,
		"model":             c.Model,
		"fast_model":        c.FastModel,
		"api_base":          c.APIBase,
		"api_key_set":       c.APIKey != "",
		"api_key_masked":    key,
		"timeout_sec":       c.TimeoutSec,
		"ollama_local_model": c.OllamaLocalModel,
		"ollama_cloud_model": c.OllamaCloudModel,
		"openai_model":       c.OpenAIModel,
		"groq_model":         c.GroqModel,
		"anthropic_model":    c.AnthropicModel,
		"gemini_model":       c.GeminiModel,
		"config_path":        llmConfigPath,
		"providers": []map[string]string{
			{"id": string(ProviderOllama), "name": ProviderOllama.DisplayName()},
			{"id": string(ProviderOpenAI), "name": ProviderOpenAI.DisplayName()},
			{"id": string(ProviderGroq), "name": ProviderGroq.DisplayName()},
			{"id": string(ProviderAnthropic), "name": ProviderAnthropic.DisplayName()},
			{"id": string(ProviderGemini), "name": ProviderGemini.DisplayName()},
		},
	})
}

func handleLLMConfigSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Provider         string `json:"provider"`
		Model            string `json:"model"`
		FastModel        string `json:"fast_model"`
		APIKey           string `json:"api_key"`
		APIBase          string `json:"api_base"`
		TimeoutSec       int    `json:"timeout_sec"`
		OllamaLocalModel string `json:"ollama_local_model"`
		OllamaCloudModel string `json:"ollama_cloud_model"`
		OpenAIModel      string `json:"openai_model"`
		GroqModel        string `json:"groq_model"`
		AnthropicModel   string `json:"anthropic_model"`
		GeminiModel      string `json:"gemini_model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	cur := getLLMConfig()
	providerChanged := req.Provider != "" && req.Provider != cur.Provider

	// If the provider changed, reset the active model/fast model/base/key to
	// that provider's sensible defaults (the per-provider presets are kept).
	if providerChanged {
		cur.Provider = req.Provider
		cur.APIKey = ""
		cur.APIBase = ""
		switch LLMProvider(req.Provider) {
		case ProviderOllama:
			cur.APIBase = "http://localhost:11434"
			cur.Model = cur.OllamaCloudModel
			cur.FastModel = cur.OllamaLocalModel
		case ProviderOpenAI:
			cur.APIBase = "https://api.openai.com/v1"
			cur.Model = cur.OpenAIModel
			cur.FastModel = cur.OpenAIModel
		case ProviderGroq:
			cur.APIBase = "https://api.groq.com/openai/v1"
			cur.Model = cur.GroqModel
			cur.FastModel = cur.GroqModel
		case ProviderAnthropic:
			cur.APIBase = "https://api.anthropic.com/v1"
			cur.Model = cur.AnthropicModel
			cur.FastModel = cur.AnthropicModel
		case ProviderGemini:
			cur.APIBase = "https://generativelanguage.googleapis.com/v1beta"
			cur.Model = cur.GeminiModel
			cur.FastModel = cur.GeminiModel
		}
	}

	// Allow blank fields to keep existing values; except provider/model which
	// should update when provided.
	if req.Provider != "" && !providerChanged {
		cur.Provider = req.Provider
	}
	if req.Model != "" {
		cur.Model = req.Model
	}
	if req.FastModel != "" {
		cur.FastModel = req.FastModel
	}
	if req.APIKey != "" {
		cur.APIKey = req.APIKey
	}
	// Support clearing key with literal "__clear__"
	if req.APIKey == "__clear__" {
		cur.APIKey = ""
	}
	if req.APIBase != "" {
		cur.APIBase = req.APIBase
	}
	if req.TimeoutSec > 0 {
		cur.TimeoutSec = req.TimeoutSec
	}
	if req.OllamaLocalModel != "" {
		cur.OllamaLocalModel = req.OllamaLocalModel
	}
	if req.OllamaCloudModel != "" {
		cur.OllamaCloudModel = req.OllamaCloudModel
	}
	if req.OpenAIModel != "" {
		cur.OpenAIModel = req.OpenAIModel
	}
	if req.GroqModel != "" {
		cur.GroqModel = req.GroqModel
	}
	if req.AnthropicModel != "" {
		cur.AnthropicModel = req.AnthropicModel
	}
	if req.GeminiModel != "" {
		cur.GeminiModel = req.GeminiModel
	}

	if err := setLLMConfig(cur); err != nil {
		http.Error(w, "could not save config: "+err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "saved", "path": llmConfigPath})
}
