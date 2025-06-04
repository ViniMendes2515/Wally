package domain

import "time"

// KnowledgeEntry representa uma entrada de conhecimento aprendida pelo sistema.
type KnowledgeEntry struct {
	ID                  int
	UserID              string
	OriginalQuery       string
	ClarificationQuery  string
	ResultingAction     string
	ResultingParameters map[string]string
	Timestamp           time.Time
}
