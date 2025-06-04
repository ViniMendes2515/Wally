package sessions

import (
	"strings"
	"sync"
)

var (
	userSessions = make(map[string]string)
	mu           sync.RWMutex // Mutex para proteger o acesso concorrente ao mapa
)

// Set armazena um valor de sessão para uma chave (número de telefone).
func Set(key, value string) {
	mu.Lock()
	defer mu.Unlock()
	userSessions[key] = value
}

// Get recupera um valor de sessão. Retorna o valor e um booleano indicando se foi encontrado.
func Get(key string) (string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	value, ok := userSessions[key]
	return value, ok
}

// Delete remove uma sessão.
func Delete(key string) {
	mu.Lock()
	defer mu.Unlock()
	delete(userSessions, key)
}

// GetAndClearIfPrefix recupera e remove uma sessão se ela começar com o prefixo especificado.
// Retorna o valor completo da sessão (fullStateValue) e a parte da string após o prefixo (contentAfterPrefix).
// Se não corresponder ao prefixo ou não existir, retorna strings vazias para ambos.
func GetAndClearIfPrefix(key, prefix string) (fullStateValue string, contentAfterPrefix string) {
	mu.Lock() // Precisa de Lock pois pode deletar
	defer mu.Unlock()

	val, ok := userSessions[key]
	if ok && strings.HasPrefix(val, prefix) {
		originalContent := strings.TrimPrefix(val, prefix)
		delete(userSessions, key)
		return val, originalContent
	}
	return "", ""
}
