package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	"ai-api-gateway/internal/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Engine handles token metering, cost calculation, and balance operations.
type Engine struct {
	db    *gorm.DB
	redis *redis.Client
}

// PriceRow contains model pricing info from DB.
type PriceRow struct {
	ID         uuid.UUID
	InputPrice  float64 // per 1K tokens (USD)
	OutputPrice float64 // per 1K tokens (USD)
	Multiplier  float64 // platform markup
}

// NewEngine creates a billing engine.
func NewEngine(db *gorm.DB, rdb *redis.Client) *Engine {
	return &Engine{db: db, redis: rdb}
}

// GetModelPrice fetches pricing info for a model by ID.
func (e *Engine) GetModelPrice(modelID string) (*PriceRow, error) {
	var row PriceRow
	parsedID, err := uuid.Parse(modelID)
	if err != nil {
		return nil, fmt.Errorf("invalid model ID: %w", err)
	}
	if err := e.db.Model(&models.Model{}).Where("id = ?", parsedID).First(&row).Error; err != nil {
		return nil, fmt.Errorf("model not found: %w", err)
	}
	return &row, nil
}

// CalculateCost computes the cost in platform units (USD).
// Formula: (promptTokens/1000 * inputPrice + completionTokens/1000 * outputPrice) * multiplier
func CalculateCost(promptTokens, completionTokens int, inputPrice, outputPrice, multiplier float64) float64 {
	promptCost := (float64(promptTokens) / 1000.0) * inputPrice
	completionCost := (float64(completionTokens) / 1000.0) * outputPrice
	cost := (promptCost + completionCost) * multiplier
	// Round to 8 decimal places
	return math.Round(cost*1e8) / 1e8
}

// PreCheckBalance checks if user has sufficient balance for estimated cost.
// Returns an error with a user-friendly message if insufficient.
func (e *Engine) PreCheckBalance(userID string, estimatedCost float64) error {
	if estimatedCost <= 0 {
		return nil
	}

	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	var user models.User
	if err := e.db.Select("balance, is_active").First(&user, "id = ?", parsedID).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	if !user.IsActive {
		return fmt.Errorf("account is disabled")
	}

	// Use Redis for hot balance if available, fall back to DB
	balance := user.Balance
	if e.redis != nil {
		hotBalance, err := e.redis.Get(context.Background(), balanceKey(userID)).Float64()
		if err == nil {
			balance = hotBalance
		}
	}

	if balance < estimatedCost {
		return fmt.Errorf("insufficient balance: have %.8f, need %.8f", balance, estimatedCost)
	}

	return nil
}

// DeductBalance atomically deducts from user balance in Redis and asynchronously persists to DB.
// Returns the balance after deduction.
func (e *Engine) DeductBalance(userID string, amount float64) (float64, error) {
	if amount <= 0 {
		// Still get current balance
		return e.getBalance(userID)
	}

	if e.redis != nil {
		// Atomic Redis deduction
		newBalance, err := e.redis.IncrByFloat(context.Background(), balanceKey(userID), -amount).Result()
		if err != nil {
			return 0, fmt.Errorf("redis balance deduction failed: %w", err)
		}
		if newBalance < 0 {
			// Rollback: add back the amount
			e.redis.IncrByFloat(context.Background(), balanceKey(userID), amount)
			return 0, fmt.Errorf("insufficient balance after deduction")
		}
		return newBalance, nil
	}

	// Fallback: DB-only deduction (less concurrent-safe)
	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID: %w", err)
	}

	var user models.User
	if err := e.db.First(&user, "id = ?", parsedID).Error; err != nil {
		return 0, fmt.Errorf("user not found: %w", err)
	}
	if user.Balance < amount {
		return 0, fmt.Errorf("insufficient balance")
	}

	newBalance := user.Balance - amount
	if err := e.db.Model(&user).Update("balance", newBalance).Error; err != nil {
		return 0, fmt.Errorf("balance update failed: %w", err)
	}
	return newBalance, nil
}

// RecordBilling creates a billing record and updates user totals in DB.
func (e *Engine) RecordBilling(userID, modelID, requestID string, promptTokens, completionTokens, totalTokens int, cost float64, requestNote string) (*models.BillingRecord, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	var parsedRequestID *uuid.UUID
	if requestID != "" {
		rid, err := uuid.Parse(requestID)
		if err == nil {
			parsedRequestID = &rid
		}
	}

	// Get balance before (from Redis if available, else from DB)
	balanceBefore, _ := e.getBalance(userID)

	desc := fmt.Sprintf("Chat completion: %d prompt + %d completion tokens", promptTokens, completionTokens)
	if requestNote != "" {
		desc += " (" + requestNote + ")"
	}

	record := &models.BillingRecord{
		UserID:       parsedUserID,
		RequestID:    parsedRequestID,
		Type:         "chat_completion",
		Amount:       -cost,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceBefore - cost,
		Description:  &desc,
	}

	if err := e.db.Create(record).Error; err != nil {
		return nil, fmt.Errorf("failed to create billing record: %w", err)
	}

	// Update user totals
	updateUserTotals(e.db, parsedUserID, cost, totalTokens)

	return record, nil
}

// InitBalance initializes a user's balance in Redis (called on login/user creation).
func (e *Engine) InitBalance(userID string) error {
	if e.redis == nil {
		return nil
	}

	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return err
	}

	var user models.User
	if err := e.db.Select("balance").First(&user, "id = ?", parsedID).Error; err != nil {
		return err
	}

	return e.redis.Set(context.Background(), balanceKey(userID), user.Balance, 0).Err()
}

// SyncBalanceToDB persists the Redis hot balance back to the database.
func (e *Engine) SyncBalanceToDB(userID string) error {
	if e.redis == nil {
		return nil
	}

	hotBalance, err := e.redis.Get(context.Background(), balanceKey(userID)).Float64()
	if err != nil {
		return err
	}

	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return err
	}

	return e.db.Model(&models.User{}).Where("id = ?", parsedID).Update("balance", hotBalance).Error
}

func (e *Engine) getBalance(userID string) (float64, error) {
	if e.redis != nil {
		balance, err := e.redis.Get(context.Background(), balanceKey(userID)).Float64()
		if err == nil {
			return balance, nil
		}
	}

	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return 0, err
	}
	var user models.User
	if err := e.db.Select("balance").First(&user, "id = ?", parsedID).Error; err != nil {
		return 0, err
	}
	return user.Balance, nil
}

func balanceKey(userID string) string {
	return "balance:" + userID
}

func updateUserTotals(db *gorm.DB, userID uuid.UUID, cost float64, tokens int) {
	db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"total_spent":  gorm.Expr("total_spent + ?", cost),
		"request_count": gorm.Expr("request_count + 1"),
	})
}

// ---- Request logging ----

// LogRequest saves a request record to the database.
func LogRequest(db *gorm.DB, req *models.Request) error {
	return db.Create(req).Error
}

// ---- Metadata helpers ----

// Metadata is a helper to create JSON metadata for billing records.
func Metadata(v interface{}) *[]byte {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return &data
}

// ---- Periodic sync ----

// StartPeriodicSync runs a goroutine that periodically syncs Redis balances to DB.
func (e *Engine) StartPeriodicSync(interval time.Duration, ctx context.Context) {
	if e.redis == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				e.syncAllBalances()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (e *Engine) syncAllBalances() {
	var userIDs []string
	e.db.Model(&models.User{}).Pluck("id", &userIDs)

	for _, uid := range userIDs {
		if err := e.SyncBalanceToDB(uid); err != nil {
			log.Printf("Balance sync failed for user %s: %v", uid, err)
		}
	}
}
