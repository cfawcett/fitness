package home

import (
	"fitness/web/components"

	"github.com/gin-gonic/gin"
)

// Handler for our home page.
func Handler(ctx *gin.Context) {
	err := components.HomePage().Render(ctx.Request.Context(), ctx.Writer)
	if err != nil {
		return
	}
}
