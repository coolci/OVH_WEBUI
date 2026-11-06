package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalogwatch"
	"net/http"
)

func GetCatalogWatch(s *app.State) gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusOK, catalogwatch.For(s).Snapshot()) }
}
func SetCatalogWatch(s *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Enabled *bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.Enabled == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "必须提供 enabled 布尔值"})
			return
		}
		w := catalogwatch.For(s)
		if err := w.SetEnabled(c.Request.Context(), *body.Enabled); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, w.Snapshot())
	}
}
