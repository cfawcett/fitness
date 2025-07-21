package middleware

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func CheckActiveWorkout(ctx *gin.Context) {
	session := sessions.Default(ctx)

	activeID := session.Get("active_workout_id")

	if activeID != nil {
		ctx.Set("ActiveWorkoutID", activeID)
	}

	ctx.Next()
}
