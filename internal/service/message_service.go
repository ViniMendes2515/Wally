package service

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
	"wally/config"
	"wally/internal/database"
	"wally/internal/domain"
	"wally/internal/rag"
	"wally/internal/sessions"
	"wally/internal/utils"
	"wally/pkg/wasender"
)

var knowledgeRepo rag.KnowledgeRepository

func ProcessMessage(number string, message string, name string) {

	dbConn := database.GetDB()
	if dbConn == nil {
		log.Fatal("Falha ao obter conexão com o banco de dados para message_service. Certifique-se que database.InitDB() foi chamado.")
	}
	knowledgeRepo = rag.NewPostgresKnowledgeRepository(dbConn)

	cfg := config.Load()

	learnedContext, errCtx := knowledgeRepo.RetrieveRelevantKnowledge(number, message)
	if errCtx != nil {
		log.Printf("Erro ao recuperar contexto para %s: %v", number, errCtx)
	}

	log.Printf("Processando mensagem de %s (%s): '%s' com contexto: '%s'", name, number, message, learnedContext)
	intent, err := callGeminiAPI(message, cfg.GeminiKey, learnedContext)

	if err != nil {
		log.Printf("Erro ao chamar API Gemini para %s: %v", number, err)
		wasender.SendMessage(number, "Erro ao conectar com a inteligência artificial. Tente novamente mais tarde.")
		return
	}

	log.Printf("Intenção detectada pela Gemini para %s: Ação=%s, Parâmetros=%v, Erro=%s", number, intent.Action, intent.Parameters, intent.Error)

	_, originalMessageIfClarifying := sessions.GetAndClearIfPrefix(number, "awaiting_clarification_unknown:")
	previousStateExpense, _ := sessions.Get(number)

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

			if originalMessageIfClarifying != "" {
				sessions.Set(number, "awaiting_clarification_unknown:"+message) // Tentar esclarecer a nova mensagem
			} else {
				sessions.Set(number, "awaiting_clarification_expense")
			}
			return
		}

		amountStr = strings.ReplaceAll(amountStr, ",", ".")
		re := regexp.MustCompile(`[^\d.]`)
		amountStr = re.ReplaceAllString(amountStr, "")

		amount, errConv := strconv.ParseFloat(amountStr, 64)
		if errConv != nil {
			wasender.SendMessage(number, fmt.Sprintf("O valor '%s' não parece ser um número válido. Poderia tentar novamente?", amountStr))
			if originalMessageIfClarifying != "" {
				sessions.Set(number, "awaiting_clarification_unknown:"+message)
			} else {
				sessions.Set(number, "awaiting_clarification_expense")
			}
			return
		}

		newExpense := domain.Expense{
			UserID:    number,
			Amount:    amount,
			Category:  strings.TrimSpace(category),
			Timestamp: time.Now(),
		}
		log.Printf("Despesa de R$%.2f na categoria %s salva para %s na data %s", newExpense.Amount, newExpense.Category, newExpense.UserID, newExpense.Timestamp.Format("2006-01-02 15:04:05"))

		responseText := fmt.Sprintf("✅ Despesa de R$%.2f na categoria '%s' adicionada com sucesso!", amount, newExpense.Category)
		wasender.SendMessage(number, responseText)

		clarifiedFromUnknown := originalMessageIfClarifying != ""
		clarifiedFromExpense := previousStateExpense == "awaiting_clarification_expense"

		if clarifiedFromUnknown || clarifiedFromExpense {
			entryOriginalQuery := originalMessageIfClarifying
			if clarifiedFromExpense && !clarifiedFromUnknown {
				if !clarifiedFromUnknown {
					log.Printf("RAG: Despesa adicionada após estado 'awaiting_clarification_expense', mas mensagem original não rastreada para aprendizado RAG completo neste fluxo.")
				}
			}

			if clarifiedFromUnknown {
				knowledgeEntry := domain.KnowledgeEntry{
					UserID:              number,
					OriginalQuery:       entryOriginalQuery,
					ClarificationQuery:  message,
					ResultingAction:     intent.Action,
					ResultingParameters: intent.Parameters,
				}
				if errSave := knowledgeRepo.SaveKnowledge(knowledgeEntry); errSave != nil {
					log.Printf("Erro ao salvar conhecimento para %s: %v", number, errSave)
				} else {
					log.Printf("RAG: Conhecimento salvo (unknown -> add_expense) para %s. Original: '%s', Clarificação: '%s'", number, entryOriginalQuery, message)
				}
			}
		}
		sessions.Delete(number)

	case "show_menu":
		wasender.SendMessage(number, utils.BuildMainMenu(name))
		if originalMessageIfClarifying != "" {
			knowledgeEntry := domain.KnowledgeEntry{
				UserID:              number,
				OriginalQuery:       originalMessageIfClarifying,
				ClarificationQuery:  message,
				ResultingAction:     intent.Action,
				ResultingParameters: intent.Parameters,
			}
			if errSave := knowledgeRepo.SaveKnowledge(knowledgeEntry); errSave != nil {
				log.Printf("Erro ao salvar conhecimento (menu após unknown) para %s: %v", number, errSave)
			} else {
				log.Printf("RAG: Conhecimento salvo (unknown -> show_menu) para %s. Original: '%s', Clarificação: '%s'", number, originalMessageIfClarifying, message)
			}
		}
		sessions.Delete(number)

	case "unknown_intent":
		responseText := fallbackConversationalResponse(message, cfg.GeminiKey)
		wasender.SendMessage(number, responseText)
		sessions.Set(number, "awaiting_clarification_unknown:"+message)
		log.Printf("SESSAO: Definido estado 'awaiting_clarification_unknown' para %s com mensagem: '%s'", number, message)

	default:
		log.Printf("Ação desconhecida ou não tratada da Gemini: %s", intent.Action)
		wasender.SendMessage(number, fmt.Sprintf("Desculpe %s, não consegui processar sua solicitação. Tente pedir o 'menu'.", name))
		if originalMessageIfClarifying != "" {
			sessions.Set(number, "awaiting_clarification_unknown:"+message)
			log.Printf("SESSAO: Mantido/Redefinido estado 'awaiting_clarification_unknown' para %s com nova mensagem: '%s'", number, message)
		} else {
			sessions.Delete(number)
		}
	}
}
