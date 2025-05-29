package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"wally/internal/service"
)

type WebhookPayload struct {
	Event string `json:"event"`
	Data  struct {
		Chats struct {
			Messages []struct {
				Message struct {
					Key struct {
						RemoteJid string `json:"remoteJid"`
					} `json:"key"`
					Pushname string `json:"pushName"`
					Message  struct {
						Conversation string `json:"conversation"`
					} `json:"message"`
				} `json:"message"`
			} `json:"messages"`
		} `json:"chats"`
	} `json:"data"`
}

func WebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metodo nao permitido", http.StatusMethodNotAllowed)
		return
	}

	var payload WebhookPayload

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Erro ao decodificar a mensagem", http.StatusBadRequest)
		return
	}

	if len(payload.Data.Chats.Messages) > 0 {
		msg := payload.Data.Chats.Messages[0]
		name := msg.Message.Pushname
		number := strings.Replace(msg.Message.Key.RemoteJid, "@s.whatsapp.net", "", 1)

		service.ProcessMessage(number, msg.Message.Message.Conversation, name)
	}
}
