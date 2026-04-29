package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"ai-api-gateway/internal/billing"
	"ai-api-gateway/internal/config"
	"ai-api-gateway/internal/database"
	"ai-api-gateway/internal/handlers"
	"ai-api-gateway/internal/middleware"
	"ai-api-gateway/internal/monitoring"
	"ai-api-gateway/internal/upstream"
)

func main() {
	// Load .env file if present (local dev), silently skip if missing
	_ = godotenv.Load()

	cfg := config.Load()

	// Database
	db, err := database.InitDatabase(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Database: %v", err)
	}

	redisClient, err := database.InitRedis(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Redis: %v", err)
	}

	// Migrations & seeds
	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Migration: %v", err)
	}
	if err := database.CreateAdminUser(db, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Printf("WARN: admin user: %v", err)
	}
	if err := database.SeedDefaultModels(db); err != nil {
		log.Printf("WARN: seed models: %v", err)
	}

	// Upstream pool
	pool := upstream.NewPool(db)
	go func() {
		pool.HealthCheck()
	}()

	// Billing engine
	billingEngine := billing.NewEngine(db, redisClient)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	billingEngine.StartPeriodicSync(5*time.Minute, ctx)

	// Monitoring & alerts
	monitorInterval, _ := time.ParseDuration(cfg.MonitorInterval)
	if monitorInterval < time.Minute {
		monitorInterval = 5 * time.Minute
	}
	alerter := monitoring.NewTelegramAlerter(cfg.TelegramBotToken, cfg.TelegramChatID)
	monitor := monitoring.NewMonitor(db, alerter, monitorInterval)
	monitor.Start(ctx)

	// Content filter (敏感词过滤)
	contentFilter := middleware.NewContentFilter(db)

	// Rate limiter (Redis)
	rateLimiter := middleware.NewRateLimiter(redisClient)

	// Handlers
	chatHandler := handlers.NewChatHandler(db, pool, billingEngine, alerter, contentFilter)
	playgroundHandler := handlers.NewPlaygroundHandler(db, chatHandler)
	modelsHandler := handlers.NewModelsHandler(db)
	mailCfg := handlers.MailConfig{
        Host:     cfg.SMTPHost,
        Port:     cfg.SMTPPort,
        User:     cfg.SMTPUser,
        Password: cfg.SMTPPassword,
        From:     cfg.EmailFrom,
    }
    handlers.SetGlobalDB(db)
	authHandler := handlers.NewAuthHandler(db, cfg.JWTSecret, redisClient, mailCfg)
	emailCodeHandler := handlers.NewEmailCodeHandler(db, redisClient, mailCfg)
	cronHandler := handlers.NewCronHandler(db, mailCfg, os.Getenv("INTERNAL_CRON_TOKEN"))
	apiKeyHandler := handlers.NewAPIKeyHandler(db)
	adminHandler := handlers.NewAdminHandler(db)
	userHandler := handlers.NewUserHandler(db)
	paymentHandler, err := handlers.NewPaymentHandler(db, handlers.AlipayConfig{
		NotifyURL: cfg.AlipayNotifyURL,
		ReturnURL: cfg.AlipayReturnURL,
		Sandbox:   cfg.AlipaySandbox == "true",
	}, cfg.AlipayAppID, cfg.AlipayPrivateKey, cfg.AlipayPublicKey)
	if err != nil {
		log.Printf("WARN: payment handler: %v", err)
	}

	// Gin engine
	r := gin.Default()
	r.Use(middleware.CORS())

	// Health check (deep: pings DB and Redis)
	r.GET("/health", func(c *gin.Context) {
		dbSQL, err := db.DB()
		dbOk := err == nil && dbSQL.Ping() == nil
		redisOk := redisClient.Ping(ctx).Err() == nil
		status := 200
		if !dbOk || !redisOk {
			status = 503
		}
		c.JSON(status, gin.H{
			"status":  "ok",
			"service": "ai-api-gateway",
			"checks": gin.H{
				"database": dbOk,
				"redis":    redisOk,
			},
		})
	})

	// === Auth routes (public) ===
	auth := r.Group("/v1/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.GET("/me", middleware.JWTAuth(cfg.JWTSecret), authHandler.Me)
		captchaHandler := handlers.NewCaptchaHandler(redisClient)
		auth.GET("/captcha/new", captchaHandler.GenerateCaptcha)
		auth.GET("/captcha/:id", captchaHandler.ServeCaptchaImage)
		auth.GET("/config", handlers.GetAuthConfig)
		auth.POST("/send-code", emailCodeHandler.SendCode)
		auth.POST("/change-password", middleware.JWTAuth(cfg.JWTSecret), authHandler.ChangePassword)
		auth.POST("/forgot-password", authHandler.ForgotPassword)
		auth.POST("/reset-password", authHandler.ResetPassword)
	}

	// === API Key management (JWT required) ===
	apiKeys := r.Group("/v1/api-keys")
	apiKeys.Use(middleware.JWTAuth(cfg.JWTSecret))
	{
		apiKeys.POST("", apiKeyHandler.Create)
		apiKeys.GET("", apiKeyHandler.List)
		apiKeys.DELETE("/:id", apiKeyHandler.Delete)
		apiKeys.PATCH("/:id/toggle", apiKeyHandler.Toggle)
		apiKeys.PATCH("/:id", apiKeyHandler.Update)
	}

	// === OpenAI-compatible API (API Key required) ===
	v1 := r.Group("/v1")
	v1.Use(middleware.APIKeyAuth(db))
	{
		v1.POST("/chat/completions", middleware.APIKeyRateLimit(rateLimiter), chatHandler.Handle)
		v1.GET("/models", modelsHandler.List)
	}

	// === Recharge & Payment routes (JWT required) ===
	recharge := r.Group("/v1/recharge")
	recharge.Use(middleware.JWTAuth(cfg.JWTSecret))
	{
		recharge.POST("/orders", paymentHandler.CreateOrder)
		recharge.GET("/orders", paymentHandler.ListOrders)
	}

	// Alipay notify callback (no auth — Alipay sends the request directly)
	r.POST("/v1/internal/daily-report", cronHandler.DailyReport)
	r.POST("/v1/recharge/alipay/notify", paymentHandler.AlipayNotify)
	r.GET("/v1/recharge/alipay/return", paymentHandler.AlipayReturn)

	// === Admin routes (JWT + AdminRequired) ===
	admin := r.Group("/v1/admin")
	admin.Use(middleware.JWTAuth(cfg.JWTSecret))
	admin.Use(middleware.AdminRequired())
	{
		// Dashboard
		admin.GET("/dashboard", adminHandler.DashboardStats)
		admin.GET("/profit", adminHandler.ProfitStats)

		// Users
		admin.GET("/users", adminHandler.ListUsers)
		admin.GET("/users/:id", adminHandler.GetUser)
		admin.PATCH("/users/:id", adminHandler.UpdateUser)

		// Channels
		admin.GET("/channels", adminHandler.ListChannels)
		admin.POST("/channels", adminHandler.CreateChannel)
		admin.PUT("/channels/:id", adminHandler.UpdateChannel)
		admin.DELETE("/channels/:id", adminHandler.DeleteChannel)
		admin.POST("/channels/:id/test", adminHandler.TestChannel)

		// Models
		admin.GET("/models", adminHandler.ListModels)
		admin.POST("/models", adminHandler.CreateModel)
		admin.PUT("/models/:id", adminHandler.UpdateModel)
		admin.DELETE("/models/:id", adminHandler.DeleteModel)

		// Logs
		admin.GET("/logs", adminHandler.ListLogs)
		admin.GET("/audit-logs", adminHandler.ListAuditLogs)

		// Recharge orders
		admin.GET("/recharge-orders", adminHandler.ListRechargeOrders)

		// Settings
		admin.GET("/settings", adminHandler.GetSettings)
		admin.PUT("/settings", adminHandler.UpdateSettings)
	}

	// === User Dashboard & Frontend API (JWT required) ===
	user := r.Group("/v1/user")
	user.Use(middleware.JWTAuth(cfg.JWTSecret))
	{
		user.GET("/dashboard", userHandler.Dashboard)
		user.GET("/billing", userHandler.ListBilling)
		user.GET("/billing/export", userHandler.ExportBilling)
		user.GET("/models", userHandler.ListPublicModels)
		user.GET("/usage", userHandler.UsageStats)
		user.POST("/playground/chat", playgroundHandler.PlaygroundChat)
	}

	// Start
	port := cfg.ServerPort
	if port == "" {
		port = "8080"
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Server starting on :%s", port)
		if err := r.Run(":" + port); err != nil {
			log.Fatalf("Server: %v", err)
		}
	}()

	<-quit
	log.Println("Shutting down...")
}
