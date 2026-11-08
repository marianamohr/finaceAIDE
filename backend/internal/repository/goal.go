package repository

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

type GoalRepository struct {
	db *gorm.DB
}

func NewGoalRepository(db *gorm.DB) *GoalRepository {
	return &GoalRepository{db: db}
}

func (r *GoalRepository) List(accountID uuid.UUID) ([]models.Goal, error) {
	var goals []models.Goal
	err := r.db.Where("account_id = ?", accountID).Order("created_at desc").Find(&goals).Error
	if err != nil {
		return nil, fmt.Errorf("goal repository: list: %w", err)
	}

	hasEmergencyGlobal := false
	for _, g := range goals {
		if g.GoalType == "emergency_global" {
			hasEmergencyGlobal = true
			break
		}
	}
	if !hasEmergencyGlobal {
		avg, _ := r.AvgMonthlyExpenses(accountID, 6)
		target := avg * 6
		if target == 0 {
			target = 10000
		}
		var emergency models.Goal
		err := r.db.Raw(`
			INSERT INTO goals (id, account_id, name, target_amount, goal_type, allocation_pct, created_at)
			VALUES (gen_random_uuid(), ?, 'Reserva de emergência global', ?, 'emergency_global', 0, NOW())
			ON CONFLICT (account_id) WHERE goal_type = 'emergency_global' DO NOTHING
			RETURNING *
		`, accountID, target).Scan(&emergency).Error
		if err != nil {
			return nil, fmt.Errorf("goal repository: create default emergency goal: %w", err)
		}
		if emergency.ID != (uuid.UUID{}) {
			goals = append([]models.Goal{emergency}, goals...)
		}
	}

	return goals, nil
}

func (r *GoalRepository) Create(goal *models.Goal) error {
	if err := r.db.Create(goal).Error; err != nil {
		return fmt.Errorf("goal repository: create: %w", err)
	}
	return nil
}

func (r *GoalRepository) Update(goal *models.Goal, accountID uuid.UUID) error {
	result := r.db.Model(goal).Where("id = ? AND account_id = ?", goal.ID, accountID).
		Updates(map[string]any{
			"name":                 goal.Name,
			"target_amount":        goal.TargetAmount,
			"current_amount":       goal.CurrentAmount,
			"initial_amount":       goal.InitialAmount,
			"monthly_contribution": goal.MonthlyContribution,
			"annual_rate":          goal.AnnualRate,
			"allocation_pct":       goal.AllocationPct,
			"goal_type":            goal.GoalType,
			"institution":          goal.Institution,
			"account_type":         goal.AccountType,
			"deadline":             goal.Deadline,
			"cover_image_url":      goal.CoverImageURL,
		})
	if result.Error != nil {
		return fmt.Errorf("goal repository: update: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// AddTransaction inserts a GoalTransaction and updates the goal's current_amount atomically.
// For yield and initial sources, no main account transaction is created.
func (r *GoalRepository) AddTransaction(gtx *models.GoalTransaction) error {
	return r.db.Transaction(func(db *gorm.DB) error {
		if err := db.Create(gtx).Error; err != nil {
			return fmt.Errorf("goal repository: add transaction: create: %w", err)
		}
		result := db.Model(&models.Goal{}).
			Where("id = ? AND account_id = ?", gtx.GoalID, gtx.AccountID).
			UpdateColumn("current_amount", gorm.Expr("current_amount + ?", gtx.Amount))
		if result.Error != nil {
			return fmt.Errorf("goal repository: add transaction: update balance: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("goal repository: add transaction: %w", ErrNotFound)
		}
		return nil
	})
}

// DepositAndRecord creates a GoalTransaction for the deposit plus a matching expense
// on the main account (money leaving the current account into the reserve).
// Used for source="manual" and source="chat" — not for yield/initial.
func (r *GoalRepository) DepositAndRecord(gtx *models.GoalTransaction, expense *models.Transaction) error {
	return r.db.Transaction(func(db *gorm.DB) error {
		if err := db.Create(gtx).Error; err != nil {
			return fmt.Errorf("goal repository: deposit: create goal tx: %w", err)
		}
		result := db.Model(&models.Goal{}).
			Where("id = ? AND account_id = ?", gtx.GoalID, gtx.AccountID).
			UpdateColumn("current_amount", gorm.Expr("current_amount + ?", gtx.Amount))
		if result.Error != nil {
			return fmt.Errorf("goal repository: deposit: update balance: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("goal repository: deposit: %w", ErrNotFound)
		}
		if err := db.Create(expense).Error; err != nil {
			return fmt.Errorf("goal repository: deposit: create expense: %w", err)
		}
		return nil
	})
}

// WithdrawAndRecord creates a GoalTransaction for the withdrawal plus the two matching
// Transactions on the main account (entry + expense), all in a single DB transaction.
func (r *GoalRepository) WithdrawAndRecord(
	gtx *models.GoalTransaction,
	entry *models.Transaction,
	expense *models.Transaction,
) error {
	return r.db.Transaction(func(db *gorm.DB) error {
		if err := db.Create(gtx).Error; err != nil {
			return fmt.Errorf("goal repository: withdraw: create goal tx: %w", err)
		}
		result := db.Model(&models.Goal{}).
			Where("id = ? AND account_id = ?", gtx.GoalID, gtx.AccountID).
			UpdateColumn("current_amount", gorm.Expr("current_amount + ?", gtx.Amount))
		if result.Error != nil {
			return fmt.Errorf("goal repository: withdraw: update balance: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("goal repository: withdraw: %w", ErrNotFound)
		}
		if err := db.Create(entry).Error; err != nil {
			return fmt.Errorf("goal repository: withdraw: create entry: %w", err)
		}
		if err := db.Create(expense).Error; err != nil {
			return fmt.Errorf("goal repository: withdraw: create expense: %w", err)
		}
		return nil
	})
}

func (r *GoalRepository) ListTransactions(goalID, accountID uuid.UUID) ([]models.GoalTransaction, error) {
	var txs []models.GoalTransaction
	err := r.db.Where("goal_id = ? AND account_id = ?", goalID, accountID).
		Order("date desc").
		Find(&txs).Error
	if err != nil {
		return nil, fmt.Errorf("goal repository: list transactions: %w", err)
	}
	return txs, nil
}

// SuggestedContribution holds the computed monthly deposit suggestion for one goal.
type SuggestedContribution struct {
	GoalID        uuid.UUID `json:"goal_id"`
	GoalName      string    `json:"goal_name"`
	SuggestedAmt  float64   `json:"suggested_amount"`
	AllocationPct float64   `json:"allocation_pct"`
}

// SuggestedContributions calculates how much to deposit into each goal this month.
// It sums the month's income from `transactions`, multiplies by the account's future_pct
// from `user_settings`, then distributes proportionally by each goal's allocation_pct.
func (r *GoalRepository) SuggestedContributions(accountID uuid.UUID, year, month int) ([]SuggestedContribution, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	var incomeRow struct{ Total float64 }
	err := r.db.Model(&models.Transaction{}).
		Select("COALESCE(SUM(amount), 0) as total").
		Where("account_id = ? AND date >= ? AND date < ? AND amount > 0", accountID, start, end).
		Scan(&incomeRow).Error
	if err != nil {
		return nil, fmt.Errorf("goal repository: suggestions: income query: %w", err)
	}

	var settings models.UserSettings
	err = r.db.Where("user_id IN (SELECT id FROM users WHERE account_id = ?)", accountID).
		First(&settings).Error
	if err != nil {
		// Fall back to 30% if settings not found
		settings.FuturePct = 0.30
	}

	futureBudget := incomeRow.Total * settings.FuturePct

	goals, err := r.List(accountID)
	if err != nil {
		return nil, err
	}

	suggestions := make([]SuggestedContribution, 0, len(goals))
	for _, g := range goals {
		if g.AllocationPct <= 0 {
			continue
		}
		suggestions = append(suggestions, SuggestedContribution{
			GoalID:        g.ID,
			GoalName:      g.Name,
			SuggestedAmt:  futureBudget * g.AllocationPct,
			AllocationPct: g.AllocationPct,
		})
	}
	return suggestions, nil
}

// GoalMonthlyPoint holds the cumulative reserve balance at the end of a given month.
type GoalMonthlyPoint struct {
	Year  int     `json:"year"`
	Month int     `json:"month"`
	Total float64 `json:"total"`
}

// GoalMonthlyHistory returns the total reserve balance at the end of each of the last
// `months` calendar months. It anchors on the live SUM(goals.current_amount) — the same
// source the KPI card uses — and reconstructs each past month by subtracting the goal
// contributions made after that month. This avoids double-counting: the previous version
// summed goal_transactions AND initial_amount account-wide, which inflated the total
// whenever the same reserve was represented both as an initial_amount on one goal and as
// a contribution (goal_transaction) on another.
func (r *GoalRepository) GoalMonthlyHistory(accountID uuid.UUID, months int) ([]GoalMonthlyPoint, error) {
	now := time.Now().UTC()
	points := make([]GoalMonthlyPoint, months)
	for i := range months {
		d := now.AddDate(0, -(months - 1 - i), 0)
		endOfMonth := time.Date(d.Year(), d.Month()+1, 1, 0, 0, 0, 0, time.UTC)

		var row struct{ Total float64 }
		err := r.db.Raw(`
			SELECT
				COALESCE((
					SELECT SUM(current_amount)
					FROM goals
					WHERE account_id = ?
				), 0)
				-
				COALESCE((
					SELECT SUM(gt.amount)
					FROM goal_transactions gt
					JOIN goals g ON g.id = gt.goal_id
					WHERE g.account_id = ? AND gt.date >= ?
				), 0) AS total
		`, accountID, accountID, endOfMonth).Scan(&row).Error
		if err != nil {
			return nil, fmt.Errorf("goal repository: monthly history: %w", err)
		}
		points[i] = GoalMonthlyPoint{Year: d.Year(), Month: int(d.Month()), Total: row.Total}
	}
	return points, nil
}

// AvgMonthlyExpenses returns the average monthly absolute expenses over the last `months` months.
// Used to compute the emergency global reserve target (6× average).
func (r *GoalRepository) AvgMonthlyExpenses(accountID uuid.UUID, months int) (float64, error) {
	now := time.Now().UTC()

	var rows []struct {
		Total float64
	}
	for i := 1; i <= months; i++ {
		d := now.AddDate(0, -i, 0)
		start := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, 0)

		var row struct{ Total float64 }
		err := r.db.Model(&models.Transaction{}).
			Select("COALESCE(SUM(amount), 0) as total").
			Where("account_id = ? AND date >= ? AND date < ? AND amount < 0", accountID, start, end).
			Scan(&row).Error
		if err != nil {
			return 0, fmt.Errorf("goal repository: avg expenses: month %d: %w", i, err)
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return 0, nil
	}
	var sum float64
	for _, row := range rows {
		sum += row.Total
	}
	avg := sum / float64(len(rows))
	if avg < 0 {
		avg = -avg
	}
	return avg, nil
}
