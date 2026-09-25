package strategy

import (
	"github.com/praxisllmlab/tianjiLLM/internal/router"
)

// SpendQuerier provides current spend data for budget decisions.
type SpendQuerier interface {
	GetProviderSpend(provider string) float64
}

// BudgetLimiter filters out deployments whose provider budget is exhausted,
// then delegates to an inner strategy for selection.
// Satisfies Strategy interface — no interface change needed.
type BudgetLimiter struct {
	budgets      map[string]float64 // provider → max budget
	inner        router.Strategy
	spendQuerier SpendQuerier
}

// NewBudgetLimiter creates a budget-limited strategy.
func NewBudgetLimiter(budgets map[string]float64, inner router.Strategy, sq SpendQuerier) *BudgetLimiter {
	return &BudgetLimiter{
		budgets:      budgets,
		inner:        inner,
		spendQuerier: sq,
	}
}

func (bl *BudgetLimiter) Pick(deployments []*router.Deployment) *router.Deployment {
	if len(deployments) == 0 {
		return nil
	}

	// Filter out deployments whose provider spend >= budget
	available := make([]*router.Deployment, 0, len(deployments))
	for _, d := range deployments {
		provider := d.ProviderName
		budget, hasBudget := bl.budgets[provider]
		if !hasBudget {
			available = append(available, d)
			continue
		}

		currentSpend := bl.spendQuerier.GetProviderSpend(provider)
		if currentSpend < budget {
			available = append(available, d)
		}
	}

	if len(available) == 0 {
		return nil // all providers exhausted
	}

	return bl.inner.Pick(available)
}
