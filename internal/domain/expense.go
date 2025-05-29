package domain

import "time"

type Expense struct {
	ID        int64
	UserID    string
	Amount    float64
	Category  string
	Timestamp time.Time
}

type ExpenseRepository interface {
	Create(expense *Expense) error
	GetAll() ([]Expense, error)
}
