package user

import (
	"fitness/platform/database"
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func getThemeForMilestone(count int) string {
	if count >= 100 {
		return "black-100" // Example
	}
	if count >= 50 {
		return "green-50"
	}
	if count >= 25 {
		return "purple-25"
	}
	return "default"
}

// UserHandler for our logged-in user page.
func UserHandler(activityRepo *database.ActivityRepo, userRepo *database.UserRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {

		sessionUserId := sessions.Default(ctx).Get("user").(uint)
		sessionUser, err := userRepo.GetUserById(uint64(sessionUserId))
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
		}
		session := sessions.Default(ctx)
		activeID := session.Get("active_workout_id")

		activityList, err := activityRepo.GetActivitiesByUserID(sessionUser.ID)
		if err != nil {
			log.Println(err)
		}
		for _, activity := range activityList {
			println(activity.ID)
			println(activity.Status)
		}

		//for i := 0; i < len(activityList); i++ {
		//	if activityList[i].Status == database.StatusDraft {
		//		activityList = append(activityList[:i], activityList[i+1:]...)
		//		i--
		//	}
		//}

		ctx.HTML(http.StatusOK, "user.html", gin.H{
			"ActiveWorkoutID": activeID,
			"User":            sessionUser,
			"ActivityList":    activityList,
		})
	}
}
