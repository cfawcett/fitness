package router

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"fitness/platform/database"
	"fitness/platform/middleware"
	"fitness/web/app/login"
	"fitness/web/app/user"
	"fitness/web/app/workout"
	"html/template"
	"os"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"fitness/platform/authenticator"
	"fitness/web/app/callback"
	"fitness/web/app/home"
)

type Handler struct {
	Router       *gin.Engine
	UserRepo     *database.UserRepo
	ActivityRepo *database.ActivityRepo
	GymSetRepo   *database.GymSetRepo
	ExerciseRepo *database.ExerciseRepo
}

// New creates the master handler with all dependencies.
func New(auth *authenticator.Authenticator) (*Handler, error) {
	db, err := database.NewDatabaseConnection()
	if err != nil {
		return nil, err
	}
	if err := database.Migrate(db); err != nil {
		return nil, err
	}

	userRepo := database.NewUserRepo(db)
	exerciseRepo := database.NewExerciseRepo(db)

	database.SeedExercises(exerciseRepo)

	engine := gin.Default()
	handler := &Handler{
		Router:       engine,
		UserRepo:     userRepo,
		ActivityRepo: database.NewActivityRepo(db),
		GymSetRepo:   database.NewGymSetRepo(db),
		ExerciseRepo: database.NewExerciseRepo(db),
	}

	engine.SetFuncMap(template.FuncMap{
		"toJSON": func(v interface{}) template.JS {
			a, _ := json.Marshal(v)
			return template.JS(a)
		},
		// This function creates a map from a list of key-value pairs
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, errors.New("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, errors.New("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
	})

	engine.LoadHTMLGlob("web/template/*.html") // Or your template path

	handler.registerRoutes(auth)

	return handler, nil
}

// registerRoutes sets up all the application's routes.
func (h *Handler) registerRoutes(auth *authenticator.Authenticator) {
	gob.Register(database.User{})
	cookieSecret := os.Getenv("COOKIE_SECRET")
	if cookieSecret == "" {
		cookieSecret = "secret" // Fallback for development, should be set in production
	}
	store := cookie.NewStore([]byte(cookieSecret))

	// Set secure and HTTP-only flags for cookies
	store.Options(sessions.Options{
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
	})

	h.Router.Use(sessions.Sessions("auth-session", store))
	h.Router.LoadHTMLGlob("web/template/*")

	h.Router.GET("/", home.Handler)
	h.Router.GET("/login", login.Handler(auth))

	h.Router.GET("/callback", callback.Handler(auth, h.UserRepo))

	h.Router.GET("/user", middleware.IsAuthenticated, middleware.CheckActiveWorkout, user.UserHandler(h.ActivityRepo))
	h.Router.POST("/workouts/new", middleware.IsAuthenticated, workout.CreateHandler(h.ActivityRepo))
	h.Router.POST("/add-exercise-to-form/:id", middleware.IsAuthenticated, workout.AddExerciseToFormHandler(h.ActivityRepo, h.GymSetRepo, h.ExerciseRepo))
	h.Router.POST("/delete-exercise/:id", middleware.IsAuthenticated, workout.DeleteExerciseHandler(h.ActivityRepo, h.GymSetRepo, h.ExerciseRepo))
	h.Router.POST("/save-draft-workout/:id", middleware.IsAuthenticated, workout.SaveDraftWorkoutHandler(h.GymSetRepo, h.ActivityRepo, h.ExerciseRepo))
	h.Router.POST("/discard-activity/:id", middleware.IsAuthenticated, workout.DiscardActivityHandler(h.ActivityRepo, h.GymSetRepo, h.ExerciseRepo))
	h.Router.POST("/save-workout/:id", middleware.IsAuthenticated, workout.SaveWorkoutHandler(h.GymSetRepo, h.ActivityRepo))
	h.Router.GET("/ui/add-exercise-modal/:id", middleware.IsAuthenticated, workout.AddExerciseModalHandler(h.ExerciseRepo))
	h.Router.GET("/ui/exercise-list/:id", middleware.IsAuthenticated, workout.ExerciseListHandler(h.ExerciseRepo))
	h.Router.GET("/exercise-info/:exerciseID", middleware.IsAuthenticated, workout.ExerciseInfoHandler(h.ExerciseRepo, h.GymSetRepo))
	h.Router.GET("/workouts/:id/edit", middleware.IsAuthenticated, workout.EditHandler(h.ActivityRepo, h.GymSetRepo, h.ExerciseRepo))
	h.Router.GET("/workouts/:id", middleware.IsAuthenticated, workout.ViewHandler(h.ActivityRepo, h.GymSetRepo, h.ExerciseRepo))
}
