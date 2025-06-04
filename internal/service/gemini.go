package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	Role  string              `json:"role,omitempty"`
}

type GeminiRequest struct {
	Contents         []GeminiRequestContent  `json:"contents"`
	GenerationConfig *GeminiGenerationConfig `json:"generationConfig,omitempty"`
}

type GeminiGenerationConfig struct {
	ResponseMIMEType string `json:"responseMimeType,omitempty"`
}

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
}

type GeminiAPIResponse struct {
	Candidates     []GeminiCandidate `json:"candidates"`
	PromptFeedback map[string]any    `json:"promptFeedback,omitempty"`
}

type IntentResponse struct {
	Action     string            `json:"action"`
	Parameters map[string]string `json:"parameters"`
	Error      string            `json:"error,omitempty"`
}

func callGeminiAPI(userMessage string, geminiKey string, learnedContext string) (IntentResponse, error) {
	var intentResp IntentResponse
	apiURL := "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash-latest:generateContent?key=" + geminiKey

	basePrompt := `
Analise a seguinte mensagem do usuário para um bot de finanças pessoais.
Extraia a intenção principal e quaisquer parâmetros relevantes.
Responda APENAS com um objeto JSON no seguinte formato:
{
  "action": "SUA_ACAO_DETECTADA",
  "parameters": {
    "amount": "valor_da_despesa",
    "category": "categoria_da_despesa",
    "description": "descricao_detalhada_da_despesa"
  },
  "error": "mensagem_de_erro_se_houver"
}

Ações possíveis e seus parâmetros:
- "add_expense": Adicionar uma nova despesa.
  - Parâmetros esperados: "amount" (número como string, ex: "100.50"), "category" (texto, ex: "lazer"), "description" (texto opcional, ex: "Assinatura do GPT").
- "show_menu": Se o usuário pedir o menu, ajuda, ou saudações iniciais (oi, olá, etc.).
  - Sem parâmetros.
- "unknown_intent": Se a intenção não for clara, não corresponder a nenhuma ação conhecida, ou se faltarem informações cruciais.
  - Parâmetro opcional "error" com uma breve descrição do problema.

Exemplos de mensagens e respostas JSON esperadas:
1. Usuário: "adicionar despesa de 100 reais com assinatura do GPT"
   JSON: {"action": "add_expense", "parameters": {"amount": "100", "category": "Assinatura", "description": "Assinatura do GPT"}}
2. Usuário: "gastei 25.50 com café"
   JSON: {"action": "add_expense", "parameters": {"amount": "25.50", "category": "café", "description": "café"}}
3. Usuário: "menu"
   JSON: {"action": "show_menu", "parameters": {}}
4. Usuário: "quero ver meu saldo"
   JSON: {"action": "unknown_intent", "parameters": {}, "error": "Funcionalidade 'ver saldo' ainda não suportada."}
`
	finalPrompt := basePrompt
	if learnedContext != "" {
		finalPrompt = fmt.Sprintf("Contexto aprendido de interações anteriores (use isso para ajudar a entender a mensagem atual):\n%s\n\n%s", learnedContext, basePrompt)
		log.Printf("GEMINI: Usando contexto aprendido: %s", learnedContext)
	}

	finalPrompt += fmt.Sprintf("\nMensagem do usuário: \"%s\"", userMessage)

	requestPayload := GeminiRequest{
		Contents: []GeminiRequestContent{
			{
				Parts: []GeminiRequestPart{
					{Text: finalPrompt},
				},
			},
		},
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
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("Erro da API Gemini - Status: %s, Body: %s", resp.Status, string(bodyBytes))
		var errorBody map[string]any
		if json.Unmarshal(bodyBytes, &errorBody) == nil {
			return intentResp, fmt.Errorf("API Gemini retornou status não OK: %s. Detalhes: %v", resp.Status, errorBody)
		}
		return intentResp, fmt.Errorf("API Gemini retornou status não OK: %s. Detalhes: %s", resp.Status, string(bodyBytes))
	}

	var geminiAPIResp GeminiAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiAPIResp); err != nil {
		return intentResp, fmt.Errorf("erro ao decodificar resposta da Gemini: %w", err)
	}

	if len(geminiAPIResp.Candidates) == 0 || len(geminiAPIResp.Candidates[0].Content.Parts) == 0 {
		log.Println("Resposta da Gemini não contém candidatos ou partes válidas.")
		log.Printf("Resposta completa da Gemini: %+v", geminiAPIResp)
		intentResp.Action = "unknown_intent"
		intentResp.Error = "Resposta da IA está vazia ou malformada."
		return intentResp, fmt.Errorf("resposta da Gemini malformada ou vazia")
	}

	responseText := geminiAPIResp.Candidates[0].Content.Parts[0].Text
	log.Printf("Texto recebido da Gemini (esperado JSON): %s", responseText)

	if err := json.Unmarshal([]byte(responseText), &intentResp); err != nil {
		log.Printf("Erro ao fazer unmarshal do JSON da Gemini para IntentResponse: %v. Texto recebido: %s", err, responseText)
		intentResp.Action = "unknown_intent"
		intentResp.Error = "Não consegui processar a resposta da IA. Tente ser mais específico ou peça o menu."
		return intentResp, nil
	}

	return intentResp, nil
}

func fallbackConversationalResponse(userMessage, geminiKey string) string {
	prompt := fmt.Sprintf(`Você é um assistente financeiro simpático. O usuário perguntou: "%s"
Se não for possível executar a ação, responda de forma educada, explique o que você pode fazer e sugira exemplos de comandos válidos.`, userMessage)
	resp, err := callGeminiAPI(prompt, geminiKey, "")
	if err != nil || resp.Action == "" {
		return "Desculpe, não consegui entender sua solicitação. Você pode tentar algo como: 'Adicionar despesa de 20 em comida' ou pedir o 'menu'."
	}
	if resp.Error != "" {
		return resp.Error
	}
	return "Desculpe, não consegui entender sua solicitação. Você pode tentar algo como: 'Adicionar despesa de 20 em comida' ou pedir o 'menu'."
}
