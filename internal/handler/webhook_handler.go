package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"wally/internal/service"
)

type WebhookPayload struct {
	Event     string `json:"event"`
	SessionID string `json:"sessionId"`
	Timestamp int64  `json:"timestamp"`
	Data      struct {
		Messages struct {
			Key struct {
				RemoteJid string `json:"remoteJid"`
				FromMe    bool   `json:"fromMe"`
				ID        string `json:"id"`
			} `json:"key"`
			MessageTimestamp int64  `json:"messageTimestamp"`
			PushName         string `json:"pushName"`
			Broadcast        bool   `json:"broadcast"`
			Message          struct {
				Conversation       string `json:"conversation"`
				MessageContextInfo any    `json:"messageContextInfo"`
			} `json:"message"`
			RemoteJid string `json:"remoteJid"`
			ID        string `json:"id"`
		} `json:"messages"`
	} `json:"data"`
}

func WebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metodo nao permitido", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Erro ao ler body", http.StatusInternalServerError)
		return
	}

	var payload WebhookPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		http.Error(w, "Erro ao decodificar a mensagem", http.StatusBadRequest)
		return
	}

	msg := payload.Data.Messages

	if msg.Key.FromMe {
		return
	}
	name := msg.PushName
	number := strings.Replace(msg.Key.RemoteJid, "@s.whatsapp.net", "", 1)
	text := msg.Message.Conversation

	service.ProcessMessage(number, text, name)
}
