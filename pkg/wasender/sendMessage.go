package wasender

import (
	"bytes"
	"encoding/json"
	"net/http"
	"wally/config"
)

func SendMessage(number string, message string) {
	cfg := config.Load()

	url := "https://www.wasenderapi.com/api/send-message"

	payloadMap := map[string]any{
		"to":   number,
		"text": message,
	}

	payload, err := json.Marshal(payloadMap)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		panic(err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+cfg.ApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()
}
