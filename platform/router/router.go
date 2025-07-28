package router

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"fitness/platform/database"
	"fitness/platform/middleware"
	"fitness/web/app/login"
	"fitness/web/app/logout"
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
	Router          *gin.Engine
	UserRepo        *database.UserRepo
	ActivityRepo    *database.ActivityRepo
	GymSetRepo      *database.GymSetRepo
	ExerciseRepo    *database.ExerciseRepo
	GymExerciseRepo *database.GymExerciseRepo
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
	//exerciseRepo := database.NewExerciseRepo(db)

	//database.SeedExercises(exerciseRepo)

	engine := gin.Default()
	handler := &Handler{
		Router:          engine,
		UserRepo:        userRepo,
		ActivityRepo:    database.NewActivityRepo(db),
		GymSetRepo:      database.NewGymSetRepo(db),
		ExerciseRepo:    database.NewExerciseRepo(db),
		GymExerciseRepo: database.NewGymExerciseRepo(db),
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
	h.Router.GET("/logout", logout.Handler)

	h.Router.GET("/callback", callback.Handler(auth, h.UserRepo))

	h.Router.GET("/profile", middleware.IsAuthenticated, user.ProfileHandler(h.UserRepo))
	h.Router.GET("/profile/edit", middleware.IsAuthenticated, user.EditProfileGetHandler(h.UserRepo))
	h.Router.POST("/profile/edit", middleware.IsAuthenticated, user.EditProfilePostHandler(h.UserRepo))

	//h.Router.POST("/add-exercise-to-form/:id", middleware.IsAuthenticated, workout.AddExerciseToFormHandler(h.ActivityRepo, h.GymSetRepo, h.ExerciseRepo))
	//h.Router.POST("/delete-exercise/:id", middleware.IsAuthenticated, workout.DeleteExerciseHandler(h.ActivityRepo, h.GymSetRepo, h.ExerciseRepo))
	//h.Router.POST("/save-draft-workout/:id", middleware.IsAuthenticated, workout.SaveDraftWorkoutHandler(h.GymSetRepo, h.ActivityRepo, h.ExerciseRepo))
	//h.Router.POST("/discard-activity/:id", middleware.IsAuthenticated, workout.DiscardActivityHandler(h.ActivityRepo, h.GymSetRepo, h.ExerciseRepo))
	//h.Router.POST("/save-workout/:id", middleware.IsAuthenticated, workout.SaveWorkoutHandler(h.GymSetRepo, h.ActivityRepo))
	//h.Router.GET("/ui/add-exercise-modal/:id", middleware.IsAuthenticated, workout.AddExerciseModalHandler(h.ExerciseRepo))
	//h.Router.GET("/ui/exercise-list/:id", middleware.IsAuthenticated, workout.ExerciseListHandler(h.ExerciseRepo))
	//h.Router.GET("/exercise-info/:exerciseID", middleware.IsAuthenticated, workout.ExerciseInfoHandler(h.ExerciseRepo, h.GymSetRepo, h.UserRepo))
	// --- Main Page Routes ---

	// Home/dashboard page
	h.Router.GET("/user", middleware.IsAuthenticated, middleware.CheckActiveWorkout, user.UserHandler(h.ActivityRepo, h.UserRepo))

	// Creates a new blank workout and redirects to the edit page
	h.Router.POST("/workouts/new", middleware.IsAuthenticated, workout.CreateHandler(h.ActivityRepo, h.UserRepo))

	// Loads the full workout editor page
	h.Router.GET("/workouts/:id/edit", middleware.IsAuthenticated, workout.EditHandler(h.ActivityRepo, h.GymSetRepo, h.ExerciseRepo, h.UserRepo))

	// Loads the read-only view of a completed workout
	h.Router.GET("/workouts/:id", middleware.IsAuthenticated, workout.ViewHandler(h.ActivityRepo, h.GymSetRepo, h.ExerciseRepo, h.UserRepo))

	// --- Component-Based HTMX Routes ---

	// --- Create Routes ---
	h.Router.POST("/activity/:id/add-exercise", middleware.IsAuthenticated, workout.AddExerciseToActivityHandler(h.GymExerciseRepo, h.GymSetRepo, h.ExerciseRepo))
	h.Router.POST("/gym-exercise/:id/add-set", middleware.IsAuthenticated, workout.AddSetToExerciseHandler(h.GymSetRepo))

	// --- Update Routes ---
	h.Router.PUT("/gym-set/:id", middleware.IsAuthenticated, workout.UpdateSetHandler(h.GymSetRepo))
	h.Router.PUT("/gym-exercise/:id", middleware.IsAuthenticated, workout.UpdateExerciseHandler(h.GymExerciseRepo))

	// --- Inline Editing Routes (New) ---
	h.Router.GET("/ui/activity-name/:id", middleware.IsAuthenticated, workout.GetActivityNameHandler(h.ActivityRepo))
	h.Router.POST("/activity/:id/name", middleware.IsAuthenticated, workout.UpdateActivityNameHandler(h.ActivityRepo))

	// --- Delete Routes ---
	h.Router.DELETE("/gym-set/:id", middleware.IsAuthenticated, workout.DeleteSetHandler(h.GymSetRepo))
	h.Router.DELETE("/gym-exercise/:id", middleware.IsAuthenticated, workout.DeleteExerciseHandler(h.GymExerciseRepo))
	h.Router.DELETE("/activity/:id", middleware.IsAuthenticated, workout.DeleteActivityHandler(h.ActivityRepo))

	// --- Main Workout Action Routes ---
	h.Router.POST("/activity/:id/finish", middleware.IsAuthenticated, workout.FinishWorkoutHandler(h.ActivityRepo))
	h.Router.POST("/activity/:id/discard", middleware.IsAuthenticated, workout.DiscardWorkoutHandler(h.ActivityRepo))
	h.Router.POST("/workouts/:id/create-edit-draft", middleware.IsAuthenticated, workout.CreateEditDraftHandler(h.ActivityRepo))

	// --- UI Fragment Routes ---
	h.Router.GET("/ui/add-exercise-modal/:id", middleware.IsAuthenticated, workout.AddExerciseModalHandler(h.ExerciseRepo))
	h.Router.GET("/ui/exercise-list/:id", middleware.IsAuthenticated, workout.ExerciseListHandler(h.ExerciseRepo, h.UserRepo))
	h.Router.GET("/exercise-info/:exerciseID", middleware.IsAuthenticated, workout.ExerciseInfoHandler(h.ExerciseRepo, h.GymSetRepo, h.UserRepo, h.ActivityRepo))
	h.Router.POST("/add-exercise-to-form/:id", middleware.IsAuthenticated, workout.AddExerciseToFormHandler(h.GymExerciseRepo, h.GymSetRepo, h.ExerciseRepo))
}
