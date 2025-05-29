package utils

import "fmt"

// BuildMainMenu gera a mensagem do menu principal do assistente virtual Wally.
func BuildMainMenu(name string) string {
	return fmt.Sprintf(
		"Olá %s, sou o Wally, seu assistente virtual. Como posso ajudar você hoje?\n\n"+
			"1️⃣ Adicionar Despesa\n"+
			"2️⃣ Adicionar Categoria\n"+
			"3️⃣ Ver extrato\n"+
			"4️⃣ Ajuda",
		name,
	)
}

func BuildDespesaAdd() string {
	return "Para adicionar uma despesa, por favor, informe o valor e a categoria da despesa.\n\n" +
		"Exemplo: 50.00 Alimentação"
}
