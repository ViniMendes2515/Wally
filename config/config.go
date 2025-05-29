package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/joho/godotenv"
)

type webhookPayload struct {
	URL string `json:"url"`
}

type Config struct {
	ApiKey      string `json:"apikey"`
	DatabaseUrl string `json:"database_url"`
	GeminiKey   string `json:"gemini_key"`
}

// Load carrega as variaveis de ambiente do arquivo .env
func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Erro ao carregar o arquivo .env:", err)
	}

	api := os.Getenv("API_KEY")
	if api == "" {
		log.Fatal("variavel de ambiente API_KEY nao encontrada")
	}

	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		log.Fatal("variavel de ambiente DATABASE_URL nao encontrada")
	}

	gemini := os.Getenv("GEMINI_KEY")
	if gemini == "" {
		log.Fatal("variavel de ambiente GEMINI_KEY nao encontrada")
	}

	cfg := Config{
		ApiKey:      api,
		DatabaseUrl: dbUrl,
		GeminiKey:   gemini,
	}

	return cfg
}

// StartNgrok inicia o ngrok e altera o webhook na WaSenderAPI
func StartNgrok(port string) (string, error) {
	cfg := Load()

	cmd := exec.Command("ngrok", "http", port)
	err := cmd.Start()
	if err != nil {
		return "", err
	}

	time.Sleep(2 * time.Second)

	resp, err := http.Get("http://127.0.0.1:4040/api/tunnels")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]any

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	tunnels := result["tunnels"].([]any)

	for _, t := range tunnels {
		tunnel := t.(map[string]any)
		if tunnel["proto"] == "https" {
			publicURL := tunnel["public_url"].(string)

			payload := webhookPayload{URL: publicURL + "/webhook"}
			body, _ := json.Marshal(payload)

			req, _ := http.NewRequest("POST", "https://wasenderapi.com/api/set-webhook", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+cfg.ApiKey)

			client := &http.Client{}
			resp, err := client.Do(req)

			if err != nil {
				return "", err
			}

			defer resp.Body.Close()
			fmt.Println("🔗 Webhook registrado na WaSenderAPI:", publicURL+"/webhook")
			return publicURL, nil
		}
	}

	return "", errors.New("nenhum túnel HTTPS encontrado")
}
