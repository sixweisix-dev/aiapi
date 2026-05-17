package handlers

import (
"github.com/gin-gonic/gin"
"gorm.io/gorm"

"ai-api-gateway/internal/billing"
)

type StripeHandler struct {
db     *gorm.DB
engine *billing.Engine
}

func NewStripeHandler(db *gorm.DB, engine *billing.Engine) *StripeHandler {
return &StripeHandler{db: db, engine: engine}
}

func (h *StripeHandler) stripeEnabled() bool {
var v string
h.db.Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", "stripe_enabled").Scan(&v)
return v == "true"
}

func (h *StripeHandler) GetStatus(c *gin.Context) {
c.JSON(200, gin.H{"enabled": h.stripeEnabled()})
}

func (h *StripeHandler) CreateCheckoutSession(c *gin.Context) {
c.JSON(503, gin.H{"error": "Stripe payment not configured yet"})
}

func (h *StripeHandler) HandleWebhook(c *gin.Context) {
c.JSON(200, gin.H{"received": true})
}
