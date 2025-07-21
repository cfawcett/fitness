package callback

import (
	"errors"
	"fitness/platform/database"
	"fitness/platform/models"
	"net/http"

	"gorm.io/gorm"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"fitness/platform/authenticator"
)

// Handler for our callback.
func Handler(auth *authenticator.Authenticator, userRepo *database.UserRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		if ctx.Query("state") != session.Get("state") {
			ctx.String(http.StatusBadRequest, "Invalid state parameter.")
			return
		}

		token, err := auth.Exchange(ctx.Request.Context(), ctx.Query("code"))
		if err != nil {
			ctx.String(http.StatusUnauthorized, "Failed to convert an authorization code into a token.")
			return
		}

		idToken, err := auth.VerifyIDToken(ctx.Request.Context(), token)
		if err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to verify ID Token.")
			return
		}

		var profile models.Auth0Profile
		if err := idToken.Claims(&profile); err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		user, err := userRepo.GetUserByAuthID(profile.Sub)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Creating new user
				user = &database.User{
					Auth0Sub:          profile.Sub,
					Email:             profile.Email,
					FirstName:         profile.GivenName,
					LastName:          profile.FamilyName,
					Username:          profile.Nickname,
					ProfilePictureUrl: profile.Picture,
				}

				if createErr := userRepo.CreateUser(user); createErr != nil {
					ctx.String(http.StatusInternalServerError, "Failed to create new user.")
					return
				}
			} else {
				ctx.String(http.StatusInternalServerError, "Database error.")
				return
			}
		}

		session.Set("access_token", token.AccessToken)
		session.Set("user", user)
		if err := session.Save(); err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		ctx.Redirect(http.StatusTemporaryRedirect, "/user")
	}
}
