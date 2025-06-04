package rag

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"wally/internal/domain"
)

// KnowledgeRepository define a interface para persistir e recuperar conhecimento.
type KnowledgeRepository interface {
	SaveKnowledge(entry domain.KnowledgeEntry) error
	RetrieveRelevantKnowledge(userID string, currentQuery string) (string, error)
}

// PostgresKnowledgeRepository é uma implementação do KnowledgeRepository usando PostgreSQL.
type PostgresKnowledgeRepository struct {
	db *sql.DB
}

// NewPostgresKnowledgeRepository cria uma nova instância do repositório PostgreSQL.
func NewPostgresKnowledgeRepository(db *sql.DB) KnowledgeRepository {
	return &PostgresKnowledgeRepository{db: db}
}

// SaveKnowledge salva uma nova entrada de conhecimento no PostgreSQL.
func (r *PostgresKnowledgeRepository) SaveKnowledge(entry domain.KnowledgeEntry) error {
	paramsJSON, err := json.Marshal(entry.ResultingParameters)
	if err != nil {
		return fmt.Errorf("erro ao converter parâmetros para JSON: %w", err)
	}

	query := `
    INSERT INTO knowledge_entries (user_id, original_query, clarification_query, resulting_action, resulting_parameters, timestamp)
    VALUES ($1, $2, $3, $4, $5, $6)`

	_, err = r.db.Exec(query,
		entry.UserID,
		entry.OriginalQuery,
		sql.NullString{String: entry.ClarificationQuery, Valid: entry.ClarificationQuery != ""},
		entry.ResultingAction,
		paramsJSON,
		time.Now(), // Usar o tempo atual no momento da inserção
	)

	if err != nil {
		return fmt.Errorf("erro ao salvar conhecimento no banco de dados: %w", err)
	}

	log.Printf("RAG_DB: Conhecimento salvo para UserID %s: Original='%s', Clarif='%s', Action='%s'",
		entry.UserID, entry.OriginalQuery, entry.ClarificationQuery, entry.ResultingAction)
	return nil
}

// RetrieveRelevantKnowledge recupera conhecimento relevante do PostgreSQL.
// Esta é uma implementação simples; RAGs mais avançados usariam embeddings e busca vetorial.
func (r *PostgresKnowledgeRepository) RetrieveRelevantKnowledge(userID string, currentQuery string) (string, error) {
	// Lógica de recuperação simples: pegar as últimas N interações para o usuário.
	// Em um sistema real, você selecionaria as mais relevantes para `currentQuery`
	// usando técnicas como busca full-text, similaridade de strings, ou embeddings.
	query := `
    SELECT original_query, clarification_query, resulting_action, resulting_parameters
    FROM knowledge_entries
    WHERE user_id = $1
    ORDER BY timestamp DESC
    LIMIT 3` // Pega as últimas 3 entradas, por exemplo

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return "", fmt.Errorf("erro ao buscar conhecimento no banco de dados: %w", err)
	}
	defer rows.Close()

	var relevantContext strings.Builder
	var entries []domain.KnowledgeEntry // Para reconstruir o contexto

	for rows.Next() {
		var entry domain.KnowledgeEntry
		var paramsJSON []byte
		var clarificationQuery sql.NullString

		if err := rows.Scan(&entry.OriginalQuery, &clarificationQuery, &entry.ResultingAction, &paramsJSON); err != nil {
			log.Printf("Erro ao escanear linha de conhecimento: %v", err)
			continue // Pula entradas malformadas
		}
		if clarificationQuery.Valid {
			entry.ClarificationQuery = clarificationQuery.String
		}

		if err := json.Unmarshal(paramsJSON, &entry.ResultingParameters); err != nil {
			log.Printf("Erro ao fazer unmarshal dos parâmetros do JSON do BD: %v", err)
			// Continuar mesmo se os parâmetros não puderem ser decodificados,
			// o resto da entrada ainda pode ser útil.
			entry.ResultingParameters = make(map[string]string) // Define como vazio para evitar nil pointer
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("erro durante iteração das linhas de conhecimento: %w", err)
	}

	// Construir o string de contexto (invertendo a ordem para mais recente primeiro no prompt)
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		contextPiece := fmt.Sprintf("Anteriormente, quando o usuário disse algo como '%s'", entry.OriginalQuery)
		if entry.ClarificationQuery != "" {
			contextPiece += fmt.Sprintf(" e depois esclareceu com '%s'", entry.ClarificationQuery)
		}
		contextPiece += fmt.Sprintf(", a intenção foi '%s' com parâmetros '%v'.\n", entry.ResultingAction, entry.ResultingParameters)
		relevantContext.WriteString(contextPiece)
	}

	if relevantContext.Len() > 0 {
		log.Printf("RAG_DB: Contexto recuperado para UserID %s: %s", userID, relevantContext.String())
	}
	return relevantContext.String(), nil
}
