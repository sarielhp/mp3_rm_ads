package main

import (
	"encoding/json"
	"net/http"
	"time"
)

type OpenRouterModel struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Context int    `json:"context_length"`
	Pricing struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`
}

type OpenRouterResponse struct {
	Data []OpenRouterModel `json:"data"`
}

var openRouterModelsCache []OpenRouterModel
var openRouterCacheMu syncMutex

func fetchOpenRouterModels() []OpenRouterModel {
	openRouterCacheMu.Lock()
	defer openRouterCacheMu.Unlock()
	if openRouterModelsCache != nil {
		return openRouterModelsCache
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://openrouter.ai/api/v1/models")
	if err != nil {
		openRouterModelsCache = []OpenRouterModel{}
		return openRouterModelsCache
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var orResp OpenRouterResponse
		if err := json.NewDecoder(resp.Body).Decode(&orResp); err == nil {
			openRouterModelsCache = orResp.Data
			return openRouterModelsCache
		}
	}

	openRouterModelsCache = []OpenRouterModel{}
	return openRouterModelsCache
}
