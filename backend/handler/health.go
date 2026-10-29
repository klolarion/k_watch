package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Version is set at build time with -ldflags "-X .../handler.Version=...".
var Version = "dev"

type HealthzResponse struct {
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

// GET /api/healthz
func Healthz(c *gin.Context) {
	response := HealthzResponse{
		Status:    "ok",
		Message:   "K-Watch is running",
		Timestamp: time.Now().UTC(),
		Version:   Version,
	}

	c.JSON(http.StatusOK, response)
}
