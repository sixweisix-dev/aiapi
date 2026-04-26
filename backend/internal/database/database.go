package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ai-api-gateway/internal/models"
)

// DB is the global database connection
var DB *gorm.DB
var RedisClient *redis.Client

// InitDatabase initializes the database connection
func InitDatabase(databaseURL string) (*gorm.DB, error) {
	// Configure GORM logger
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  true,
		},
	)

	// Open database connection
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL DB for connection pooling
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	DB = db
	return db, nil
}

// InitRedis initializes Redis connection
func InitRedis(redisURL string) (*redis.Client, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	RedisClient = client
	return client, nil
}

// RunMigrations runs database migrations
func RunMigrations(db *gorm.DB) error {
	log.Println("Running database migrations...")

	// Enable UUID extension
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error; err != nil {
		return fmt.Errorf("failed to create uuid extension: %w", err)
	}

	// Auto migrate all models
	err := db.AutoMigrate(
		&models.User{},
		&models.APIKey{},
		&models.APIKeyAllowedModel{},
		&models.UpstreamChannel{},
		&models.Model{},
		&models.Request{},
		&models.BillingRecord{},
		&models.RechargeOrder{},
		&models.Subscription{},
		&models.AuditLog{},
	)
	if err != nil {
		return fmt.Errorf("failed to auto migrate: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}

// CreateAdminUser creates the initial admin user if it doesn't exist
func CreateAdminUser(db *gorm.DB, email, password string) error {
	// Check if admin already exists
	var count int64
	if err := db.Model(&models.User{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check admin user: %w", err)
	}

	if count > 0 {
		log.Println("Admin user already exists")
		return nil
	}

	// Hash password with bcrypt
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}
	hashedPassword := string(hashedBytes)

	adminUser := &models.User{
		Email:         email,
		PasswordHash:  hashedPassword,
		Username:      &[]string{"admin"}[0],
		Role:          "admin",
		Balance:       1000000,
		IsActive:      true,
		EmailVerified: true,
	}

	if err := db.Create(adminUser).Error; err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	log.Printf("Admin user created with email: %s", email)
	return nil
}

// SeedDefaultModels seeds default AI models
func SeedDefaultModels(db *gorm.DB) error {
	defaultModels := []models.Model{
		{
			Name:          "gpt-4",
			DisplayName:   "GPT-4",
			Provider:      "openai",
			ContextLength: 8192,
			InputPrice:    0.03,
			OutputPrice:   0.06,
			Multiplier:    1.5,
			IsEnabled:     true,
			IsPublic:      true,
			Description:   &[]string{"OpenAI GPT-4"}[0],
		},
		{
			Name:          "gpt-3.5-turbo",
			DisplayName:   "GPT-3.5 Turbo",
			Provider:      "openai",
			ContextLength: 4096,
			InputPrice:    0.0015,
			OutputPrice:   0.002,
			Multiplier:    1.5,
			IsEnabled:     true,
			IsPublic:      true,
			Description:   &[]string{"OpenAI GPT-3.5 Turbo"}[0],
		},
		{
			Name:          "claude-3-opus",
			DisplayName:   "Claude 3 Opus",
			Provider:      "anthropic",
			ContextLength: 200000,
			InputPrice:    0.015,
			OutputPrice:   0.075,
			Multiplier:    1.5,
			IsEnabled:     true,
			IsPublic:      true,
			Description:   &[]string{"Anthropic Claude 3 Opus"}[0],
		},
		{
			Name:          "claude-3-sonnet",
			DisplayName:   "Claude 3 Sonnet",
			Provider:      "anthropic",
			ContextLength: 200000,
			InputPrice:    0.003,
			OutputPrice:   0.015,
			Multiplier:    1.5,
			IsEnabled:     true,
			IsPublic:      true,
			Description:   &[]string{"Anthropic Claude 3 Sonnet"}[0],
		},
		{
			Name:          "gemini-pro",
			DisplayName:   "Gemini Pro",
			Provider:      "google",
			ContextLength: 32768,
			InputPrice:    0.0005,
			OutputPrice:   0.0015,
			Multiplier:    1.5,
			IsEnabled:     true,
			IsPublic:      true,
			Description:   &[]string{"Google Gemini Pro"}[0],
		},
	}

	for _, model := range defaultModels {
		// Check if model already exists
		var count int64
		if err := db.Model(&models.Model{}).Where("name = ?", model.Name).Count(&count).Error; err != nil {
			return fmt.Errorf("failed to check model %s: %w", model.Name, err)
		}

		if count == 0 {
			if err := db.Create(&model).Error; err != nil {
				return fmt.Errorf("failed to create model %s: %w", model.Name, err)
			}
			log.Printf("Created default model: %s", model.Name)
		}
	}

	log.Println("Default models seeded successfully")
	return nil
}