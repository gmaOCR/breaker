package policy

import (
	"fmt"

	"github.com/gmaOCR/breaker/internal/core"
)

// HardCap trips when cumulative spend reaches the USD budget or the token budget.
type HardCap struct{}

func (HardCap) Name() string { return "hardcap" }

func (HardCap) Check(s State, _ core.SpendEvent) (bool, string) {
	if s.BudgetUSD > 0 && s.SpentUSD >= s.BudgetUSD {
		return true, fmt.Sprintf("budget of $%.2f reached ($%.4f spent)", s.BudgetUSD, s.SpentUSD)
	}
	if s.TokenBudget > 0 && s.Tokens >= s.TokenBudget {
		return true, fmt.Sprintf("token budget of %d reached (%d used)", s.TokenBudget, s.Tokens)
	}
	return false, ""
}
