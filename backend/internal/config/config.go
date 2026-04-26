package config

import "os"

type Config struct {
        DatabaseURL       string
        RedisURL          string
        JWTSecret         string
        ServerPort        string
        Environment       string
        AdminEmail        string
        AdminPassword     string
        AlipayAppID       string
        AlipayPrivateKey  string
        AlipayPublicKey   string
        AlipayNotifyURL   string
        AlipayReturnURL   string
        AlipaySandbox     string
        TelegramBotToken  string
        TelegramChatID    string
        MonitorInterval   string
        SMTPHost          string
        SMTPPort          string
        SMTPUser          string
        SMTPPassword      string
        EmailFrom         string
}

func Load() *Config {
        return &Config{
                DatabaseURL:      getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/ai_gateway?sslmode=disable"),
                RedisURL:         getEnv("REDIS_URL", "redis://localhost:6379/0"),
                JWTSecret:        getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
                ServerPort:       getEnv("PORT", "8080"),
                Environment:      getEnv("ENVIRONMENT", "development"),
                AdminEmail:       getEnv("ADMIN_EMAIL", "admin@example.com"),
                AdminPassword:    getEnv("ADMIN_PASSWORD", "admin123"),
                AlipayAppID:      getEnv("ALIPAY_APP_ID", ""),
                AlipayPrivateKey: getEnv("ALIPAY_PRIVATE_KEY", ""),
                AlipayPublicKey:  getEnv("ALIPAY_PUBLIC_KEY", ""),
                AlipayNotifyURL:  getEnv("ALIPAY_NOTIFY_URL", "https://your-domain.com/v1/recharge/alipay/notify"),
                AlipayReturnURL:  getEnv("ALIPAY_RETURN_URL", "https://your-domain.com/v1/recharge/alipay/return"),
                AlipaySandbox:    getEnv("ALIPAY_SANDBOX", "true"),
                TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
                TelegramChatID:   getEnv("TELEGRAM_CHAT_ID", ""),
                MonitorInterval:  getEnv("MONITOR_INTERVAL", "5m"),
                SMTPHost:         getEnv("SMTP_HOST", "smtp.gmail.com"),
                SMTPPort:         getEnv("SMTP_PORT", "587"),
                SMTPUser:         getEnv("SMTP_USER", ""),
                SMTPPassword:     getEnv("SMTP_PASSWORD", ""),
                EmailFrom:        getEnv("EMAIL_FROM", ""),
        }
}

func getEnv(key, fallback string) string {
        if v := os.Getenv(key); v != "" {
                return v
        }
        return fallback
}
