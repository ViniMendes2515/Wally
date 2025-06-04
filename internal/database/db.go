package database

import (
	"database/sql"
	"fmt"
	"log"
	"wally/config"

	_ "github.com/lib/pq"
)

var db *sql.DB

// InitDB inicializa a conexão com o banco de dados PostgreSQL.
func InitDB() error {
	cfg := config.Load()
	connStr := cfg.DatabaseUrl

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("erro ao abrir conexão com o banco de dados: %w", err)
	}

	err = db.Ping()
	if err != nil {
		db.Close()
		return fmt.Errorf("erro ao conectar com o banco de dados (ping): %w", err)
	}

	log.Println("Conexão com o banco de dados PostgreSQL estabelecida com sucesso!")
	return createKnowledgeTableIfNotExists()
}

// GetDB retorna a instância da conexão com o banco de dados.
func GetDB() *sql.DB {
	if db == nil {
		log.Fatal("A conexão com o banco de dados não foi inicializada. Chame InitDB primeiro.")
	}
	return db
}

// createKnowledgeTableIfNotExists cria a tabela para armazenar o conhecimento, se ela não existir.
func createKnowledgeTableIfNotExists() error {
	query := `
    CREATE TABLE IF NOT EXISTS knowledge_entries (
        id SERIAL PRIMARY KEY,
        user_id VARCHAR(255) NOT NULL,
        original_query TEXT,
        clarification_query TEXT,
        resulting_action VARCHAR(255),
        resulting_parameters JSONB, -- Usar JSONB para armazenar os parâmetros
        timestamp TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
    );`

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("erro ao criar tabela knowledge_entries: %w", err)
	}
	log.Println("Tabela 'knowledge_entries' verificada/criada com sucesso.")
	return nil
}
