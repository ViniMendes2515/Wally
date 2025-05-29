package service

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
	"wally/config"
	"wally/internal/domain"
	"wally/internal/sessions"
	"wally/internal/utils"
	"wally/pkg/wasender"
)

func ProcessMessage(number string, message string, name string) {
	cfg := config.Load()

	log.Printf("Processando mensagem de %s (%s): '%s'", name, number, message)
	intent, err := callGeminiAPI(message, cfg.GeminiKey)

	if err != nil {
		log.Printf("Erro ao chamar API Gemini para %s: %v", number, err)
		wasender.SendMessage(number, "Erro conectar com a inteligência artificial. Tente novamente mais tarde.")
		return
	}

	log.Printf("Intenção detectada pela Gemini para %s: Ação=%s, Parâmetros=%v, Erro=%s", number, intent.Action, intent.Parameters, intent.Error)

	switch intent.Action {
	case "add_expense":
		amountStr, okAmount := intent.Parameters["amount"]
		category, okCategory := intent.Parameters["category"]

		if !okAmount || !okCategory || amountStr == "" || category == "" {
			errorMsg := "Não consegui identificar o valor ou a categoria da despesa."
			if intent.Error != "" {
				errorMsg = intent.Error
			}
			wasender.SendMessage(number, fmt.Sprintf("%s Poderia tentar novamente? Ex: Adicionar despesa de 50 na categoria Lazer", errorMsg))
			sessions.Set(number, "awaiting_clarification_expense")
			return
		}

		amountStr = strings.ReplaceAll(amountStr, ",", ".")
		re := regexp.MustCompile(`[^\d.]`)
		amountStr = re.ReplaceAllString(amountStr, "")

		amount, errConv := strconv.ParseFloat(amountStr, 64)
		if errConv != nil {
			wasender.SendMessage(number, fmt.Sprintf("O valor '%s' não parece ser um número válido. Poderia tentar novamente?", amountStr))
			sessions.Set(number, "awaiting_clarification_expense")
			return
		}

		newExpense := domain.Expense{
			UserID:    number,
			Amount:    amount,
			Category:  strings.TrimSpace(category),
			Timestamp: time.Now(),
		}

		log.Printf("Despesa de R$%.2f na categoria %s salva para %s na dta %s", newExpense.Amount, newExpense.Category, newExpense.UserID, newExpense.Timestamp.Format("2006-01-02 15:04:05"))

		responseText := fmt.Sprintf("✅ Despesa de R$%.2f na categoria '%s' adicionada com sucesso!", amount, newExpense.Category)
		wasender.SendMessage(number, responseText)
		sessions.Delete(number)

	case "show_menu":
		wasender.SendMessage(number, utils.BuildMainMenu(name))

	case "unknown_intent":
		responseText := fmt.Sprintf("Desculpe %s, não entendi bem.", name)
		if intent.Error != "" {
			responseText = fmt.Sprintf("Desculpe %s, %s", name, intent.Error)
		} else {
			responseText += " Você pode tentar algo como:\n" +
				"➡️ 'Adicionar despesa de 20 em comida'\n" +
				"➡️ Ou peça o 'menu' para ver as opções."
		}
		wasender.SendMessage(number, responseText)

	default:
		log.Printf("Ação desconhecida ou não tratada da Gemini: %s", intent.Action)
		wasender.SendMessage(number, fmt.Sprintf("Desculpe %s, não consegui processar sua solicitação. Tente pedir o 'menu'.", name))
	}
}
