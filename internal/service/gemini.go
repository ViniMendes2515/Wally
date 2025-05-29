package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Estruturas para a requisição à API Gemini
type GeminiRequestPart struct {
	Text string `json:"text"`
}

type GeminiRequestContent struct {
	Parts []GeminiRequestPart `json:"parts"`
	Role  string              `json:"role,omitempty"` // "user" ou "model"
}

type GeminiRequest struct {
	Contents         []GeminiRequestContent  `json:"contents"`
	GenerationConfig *GeminiGenerationConfig `json:"generationConfig,omitempty"`
}

type GeminiGenerationConfig struct {
	ResponseMIMEType string `json:"responseMimeType,omitempty"` // Para forçar JSON output
}

// Estruturas para a resposta da API Gemini
type GeminiResponsePart struct {
	Text string `json:"text"`
}

type GeminiResponseContent struct {
	Parts []GeminiResponsePart `json:"parts"`
	Role  string               `json:"role"`
}

type GeminiCandidate struct {
	Content      GeminiResponseContent `json:"content"`
	FinishReason string                `json:"finishReason"`
	Index        int                   `json:"index"`
	// SafetyRatings []SafetyRating `json:"safetyRatings"` // Pode ser adicionado se necessário
}

type GeminiAPIResponse struct {
	Candidates     []GeminiCandidate `json:"candidates"`
	PromptFeedback map[string]any    `json:"promptFeedback,omitempty"` // Para debug
}

// IntentResponse é a estrutura que esperamos que a Gemini retorne (dentro do campo 'text' da resposta dela)
type IntentResponse struct {
	Action     string            `json:"action"`
	Parameters map[string]string `json:"parameters"`
	Error      string            `json:"error,omitempty"` // Se a Gemini detectar um erro ou não entender
}

// callGeminiAPI faz a chamada real para a API Gemini
func callGeminiAPI(userMessage string, geminiKey string) (IntentResponse, error) {
	var intentResp IntentResponse
	// Usar gemini-1.5-flash-latest que é bom para tarefas rápidas e suporta JSON output
	// Se você tiver acesso ao gemini-2.0-flash, pode ajustar o nome do modelo.
	apiURL := "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash-latest:generateContent?key=" + geminiKey

	// Prompt para instruir a Gemini a retornar um JSON
	// Este prompt é crucial para o sucesso da extração.
	prompt := fmt.Sprintf(`
Analise a seguinte mensagem do usuário para um bot de finanças pessoais.
Extraia a intenção principal e quaisquer parâmetros relevantes.
Responda APENAS com um objeto JSON no seguinte formato:
{
  "action": "SUA_ACAO_DETECTADA",
  "parameters": {
    "parametro1": "valor1",
    "parametro2": "valor2"
  },
  "error": "mensagem_de_erro_se_houver"
}

Ações possíveis e seus parâmetros:
- "add_expense": Adicionar uma nova despesa.
  - Parâmetros esperados: "amount" (número como string, ex: "100.50"), "category" (texto, ex: "lazer").
- "show_menu": Se o usuário pedir o menu, ajuda, ou saudações iniciais (oi, olá, etc.).
  - Sem parâmetros.
- "unknown_intent": Se a intenção não for clara, não corresponder a nenhuma ação conhecida, ou se faltarem informações cruciais.
  - Parâmetro opcional "error" com uma breve descrição do problema.

Exemplos de mensagens e respostas JSON esperadas:
1. Usuário: "adicionar despesa de 100 reais na categoria lazer"
   JSON: {"action": "add_expense", "parameters": {"amount": "100", "category": "lazer"}}
2. Usuário: "gastei 25.50 com café"
   JSON: {"action": "add_expense", "parameters": {"amount": "25.50", "category": "café"}}
3. Usuário: "menu"
   JSON: {"action": "show_menu", "parameters": {}}
4. Usuário: "quero ver meu saldo" (não implementado ainda, então unknown)
   JSON: {"action": "unknown_intent", "parameters": {}, "error": "Funcionalidade 'ver saldo' ainda não suportada."}
5. Usuário: "adicionar despesa de comida" (falta valor)
   JSON: {"action": "unknown_intent", "parameters": {"category": "comida"}, "error": "Valor da despesa não especificado."}


Mensagem do usuário: "%s"
`, userMessage)

	requestPayload := GeminiRequest{
		Contents: []GeminiRequestContent{
			{
				Parts: []GeminiRequestPart{
					{Text: prompt},
				},
			},
		},
		// Para modelos que suportam, forçar a saída JSON é mais robusto.
		// Gemini 1.5 Flash suporta isso.
		GenerationConfig: &GeminiGenerationConfig{
			ResponseMIMEType: "application/json",
		},
	}

	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return intentResp, fmt.Errorf("erro ao fazer marshal do payload da Gemini: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return intentResp, fmt.Errorf("erro ao criar requisição para Gemini: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return intentResp, fmt.Errorf("erro ao enviar requisição para Gemini: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errorBody)
		log.Printf("Erro da API Gemini - Status: %s, Body: %v", resp.Status, errorBody)
		return intentResp, fmt.Errorf("API Gemini retornou status não OK: %s. Detalhes: %v", resp.Status, errorBody)
	}

	var geminiAPIResp GeminiAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiAPIResp); err != nil {
		return intentResp, fmt.Errorf("erro ao decodificar resposta da Gemini: %w", err)
	}

	if len(geminiAPIResp.Candidates) == 0 || len(geminiAPIResp.Candidates[0].Content.Parts) == 0 {
		log.Println("Resposta da Gemini não contém candidatos ou partes válidas.")
		log.Printf("Resposta completa da Gemini: %+v", geminiAPIResp)
		return intentResp, fmt.Errorf("resposta da Gemini malformada ou vazia")
	}

	// O texto da Gemini que esperamos ser um JSON
	responseText := geminiAPIResp.Candidates[0].Content.Parts[0].Text
	log.Printf("Texto recebido da Gemini (esperado JSON): %s", responseText)

	// Tentar decodificar o texto da Gemini para nossa estrutura IntentResponse
	if err := json.Unmarshal([]byte(responseText), &intentResp); err != nil {
		log.Printf("Erro ao fazer unmarshal do JSON da Gemini para IntentResponse: %v. Texto recebido: %s", err, responseText)
		// Fallback se o JSON não for o esperado, mas ainda tentar uma análise básica
		intentResp.Action = "unknown_intent"
		intentResp.Error = "Não consegui processar a resposta da IA. Tente ser mais específico ou peça o menu."
		return intentResp, nil // Retorna um erro "suave" para o chamador lidar
	}

	return intentResp, nil
}
