package main

import (
	"log"
	"net/http"
	"wally/config"
	"wally/internal/database"
	"wally/internal/handler"
)

func main() {

	if err := database.InitDB(); err != nil {
		log.Fatalf("Erro ao inicializar o banco de dados: %v", err)
	}
	_, err := config.StartNgrok("8080")
	if err != nil {
		log.Fatalf("Erro ao iniciar o ngrok: %v", err)
	}

	http.HandleFunc("/webhook", handler.WebhookHandler)

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Erro ao iniciar o servidor: %v", err)
	}

	log.Println("Servidor iniciado na porta 8080")
}
