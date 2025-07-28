package user

import (
	"bytes"
	"encoding/json"
	"errors"
	"fitness/platform/auth0"
	"fitness/platform/database"
	"gorm.io/gorm"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

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

		for i := 0; i < len(activityList); i++ {
			if activityList[i].Status == database.StatusDraft {
				activityList = append(activityList[:i], activityList[i+1:]...)
				i--
			}
		}

		ctx.HTML(http.StatusOK, "user.html", gin.H{
			"ActiveWorkoutID": activeID,
			"User":            sessionUser,
			"ActivityList":    activityList,
		})
	}
}

func ProfileHandler(userRepo *database.UserRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		sessionUserId := sessions.Default(ctx).Get("user").(uint)
		sessionUser, err := userRepo.GetUserById(uint64(sessionUserId))
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
		}
		ctx.HTML(http.StatusOK, "profile.html", gin.H{
			"User": sessionUser,
		})
	}
}

// EditProfileGetHandler renders the page with the form to edit a user's profile.
func EditProfileGetHandler(userRepo *database.UserRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 1. Get the current user from the session and database
		sessionUserId := sessions.Default(ctx).Get("user").(uint)
		sessionUser, err := userRepo.GetUserById(uint64(sessionUserId))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ctx.String(http.StatusNotFound, "User not found.")
				return
			}
			ctx.String(http.StatusInternalServerError, "Could not retrieve user.")
			return
		}

		// 2. Determine if the user signed up via a social connection
		isSocialUser := !strings.HasPrefix(sessionUser.Auth0Sub, "auth0|")

		// 3. Render the edit page, passing in the user and the new flag
		ctx.HTML(http.StatusOK, "edit-profile.html", gin.H{
			"User":         sessionUser,
			"IsSocialUser": isSocialUser,
		})
	}
}

func EditProfilePostHandler(userRepo *database.UserRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 1. Get the current user from the session and database
		sessionUserId := sessions.Default(ctx).Get("user").(uint)
		sessionUser, err := userRepo.GetUserById(uint64(sessionUserId))
		if err != nil {
			ctx.String(http.StatusInternalServerError, "Could not find user.")
			return
		}

		// 2. Bind and parse all form data
		firstName := ctx.PostForm("FirstName")
		lastName := ctx.PostForm("LastName")

		// 3. Update core identity fields in Auth0 first
		token, err := auth0.GetManagementAPIToken()
		if err != nil {
			log.Printf("Failed to get Auth0 token: %v", err)
			ctx.String(http.StatusInternalServerError, "Failed to update profile.")
			return
		}

		auth0Payload := map[string]string{
			"given_name":  firstName,
			"family_name": lastName,
		}
		payloadBytes, err := json.Marshal(auth0Payload)
		if err != nil {
			log.Printf("Failed to marshal Auth0 payload: %v", err)
			ctx.String(http.StatusInternalServerError, "Failed to update profile.")
			return
		}

		domain := os.Getenv("AUTH0_DOMAIN")
		url := "https://" + domain + "/api/v2/users/" + sessionUser.Auth0Sub

		req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(payloadBytes))
		if err != nil {
			log.Printf("Failed to create Auth0 request: %v", err)
			ctx.String(http.StatusInternalServerError, "Failed to update profile.")
			return
		}
		req.Header.Add("authorization", "Bearer "+token)
		req.Header.Add("content-type", "application/json")

		res, err := http.DefaultClient.Do(req)
		if err != nil || res.StatusCode != http.StatusOK {
			log.Printf("Failed to update user in Auth0. Status: %s", res.Status)
			ctx.String(http.StatusInternalServerError, "Failed to update profile.")
			return
		}
		defer res.Body.Close()

		// 4. Update the local user object with all data from the form
		sessionUser.FirstName = firstName
		sessionUser.LastName = lastName
		sessionUser.Username = ctx.PostForm("Username") // TODO check this is globally unique
		sessionUser.Bio = ctx.PostForm("Bio")
		sessionUser.Location = ctx.PostForm("Location")

		heightStr := ctx.PostForm("HeightCM")
		if heightStr != "" {
			height, err := strconv.Atoi(heightStr)
			if err != nil {
				ctx.String(http.StatusBadRequest, "Invalid height provided.")
				return
			}
			sessionUser.HeightCM = height
		}

		weightStr := ctx.PostForm("CurrentWeightKG")
		if weightStr != "" {
			weight, err := strconv.ParseFloat(weightStr, 64)
			if err != nil {
				ctx.String(http.StatusBadRequest, "Invalid weight provided.")
				return
			}
			sessionUser.CurrentWeightKG = weight
		}

		dobStr := ctx.PostForm("Dob")
		if dobStr != "" {
			dob, err := time.Parse("2006-01-02", dobStr)
			if err != nil {
				ctx.String(http.StatusBadRequest, "Invalid date of birth provided.")
				return
			}
			sessionUser.Dob = dob
		}

		// 5. Save the updated user object to your local database
		if err := userRepo.UpdateUser(sessionUser); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to update profile in local database.")
			return
		}

		// 6. Redirect back to the profile page on success
		ctx.Redirect(http.StatusFound, "/profile")
	}
}
