package workout

import (
	"errors"
	"fitness/platform/database"
	"fmt"
	"gorm.io/gorm"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// CreateHandler handles the POST /workouts/new request
func CreateHandler(activityRepo *database.ActivityRepo, userRepo *database.UserRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		activeidInterface := session.Get("active_workout_id")
		discard := ctx.Query("discard") == "true" // Check for the discard confirmation

		if activeidInterface != nil && !discard {
			activeID := activeidInterface.(uint)

			returnURL := fmt.Sprintf("/workouts/%d/edit", activeID)
			discardAndStartNewUrl := "/workouts/new?discard=true"

			ctx.HTML(http.StatusOK, "_create_workout_confirm.html", gin.H{
				"DiscardURL": discardAndStartNewUrl,
				"ReturnURL":  returnURL,
			})
			return
		}

		sessionUserId := session.Get("user").(uint)
		sessionUser, err := userRepo.GetUserById(uint64(sessionUserId))
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		newActivity := &database.Activity{
			UserID:       sessionUser.ID,
			Type:         "GYM_WORKOUT",
			ActivityTime: time.Now(),
			Name:         "Gym Workout",
			Status:       database.StatusDraft,
		}

		if err := activityRepo.CreateActivity(newActivity); err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		if activeidInterface != nil {
			if err := activityRepo.DeleteActivity(activeidInterface.(uint)); err != nil {
				ctx.String(http.StatusInternalServerError, err.Error())
			}
		}

		session.Set("active_workout_id", newActivity.ID)
		if err := session.Save(); err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}

		redirectURL := fmt.Sprintf("/workouts/%d/edit", newActivity.ID)
		ctx.Header("HX-Redirect", redirectURL) // htmx will see this header and redirect
		ctx.Status(http.StatusOK)
	}
}

func ViewHandler(activityRepo *database.ActivityRepo, gymSetRepo *database.GymSetRepo, exerciseRepo *database.ExerciseRepo, userRepo *database.UserRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		sessionUserId := sessions.Default(ctx).Get("user").(uint)
		sessionUser, err := userRepo.GetUserById(uint64(sessionUserId))
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
		}
		idStr := ctx.Param("id")
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			ctx.String(http.StatusBadRequest, "invalid workout id")
			return
		}

		activity, err := activityRepo.GetActivityByID(uint(id))
		if err != nil {
			ctx.String(http.StatusNotFound, "Workout not found")
			return
		}

		populatedExercises, err := gymSetRepo.GetPopulatedExercises(activity.ID)
		if err != nil {
			ctx.String(http.StatusInternalServerError, "could not process workout sets")
			return
		}

		ctx.HTML(http.StatusOK, "view-workout.html", gin.H{
			"Activity":           activity,
			"PopulatedExercises": populatedExercises,
			"User":               sessionUser,
		})
	}
}

func EditHandler(activityRepo *database.ActivityRepo, gymSetRepo *database.GymSetRepo, exerciseRepo *database.ExerciseRepo, userRepo *database.UserRepo, draftRepo *database.DraftGymSetRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		sessionUserId := sessions.Default(ctx).Get("user").(uint)
		sessionUser, err := userRepo.GetUserById(uint64(sessionUserId))
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
		}
		idStr := ctx.Param("id")
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			ctx.String(http.StatusBadRequest, "invalid workout id")
			return
		}

		activity, err := activityRepo.GetActivityByID(uint(id))
		if err != nil {
			ctx.String(http.StatusNotFound, "Workout not found")
			return
		}

		allExercises, _ := exerciseRepo.GetExerciseList()

		//Does draft exist for this activity? if yes, give it the draft list, if no, give it the actual list

		draftExercises, err := draftRepo.GetPopulatedDraftExercises(activity.ID)
		if err == nil {
			ctx.HTML(http.StatusOK, "edit-workout.html", gin.H{
				"Activity":           activity,
				"PopulatedExercises": draftExercises,
				"AllExercises":       allExercises,
				"NextExerciseIndex":  len(draftExercises),
				"User":               sessionUser,
			})
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			println("no draft sets found")
			savedExercises, err := gymSetRepo.GetPopulatedExercises(activity.ID)
			if err != nil {
				ctx.String(http.StatusInternalServerError, "could not process workout sets")
			}
			ctx.HTML(http.StatusOK, "edit-workout.html", gin.H{
				"Activity":           activity,
				"PopulatedExercises": savedExercises,
				"AllExercises":       allExercises,
				"NextExerciseIndex":  len(savedExercises),
				"User":               sessionUser,
			})
		}
	}
}

type SetData struct {
	Reps     int     `form:"reps"`
	WeightKG float64 `form:"weight_kg"`
}

type ExerciseData struct {
	ExerciseID uint64    `form:"exercise_id"`
	Sets       []SetData `form:"sets"`
}

type WorkoutForm struct {
	Exercises []ExerciseData `form:"exercises"`
}

func SaveWorkoutHandler(gymSetRepo *database.GymSetRepo, activityRepo *database.ActivityRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		idStr := ctx.Param("id")
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			ctx.String(http.StatusBadRequest, "Invalid activity ID")
			return
		}

		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to parse form: %v", err)
			return
		}

		form, err := bindWorkoutFormManually(ctx.Request.PostForm)
		if err != nil {
			ctx.String(http.StatusBadRequest, "Failed to bind form manually: %v", err)
			return
		}

		var newSetsToCreate []*database.GymSet
		currentSetNumber := 0
		for _, exercise := range form.Exercises {
			for _, set := range exercise.Sets {
				currentSetNumber++
				gymSet := &database.GymSet{
					ActivityID:           uint(id),
					ExerciseDefinitionID: uint(exercise.ExerciseID),
					SetNumber:            currentSetNumber,
					Reps:                 set.Reps,
					WeightKG:             set.WeightKG,
				}
				newSetsToCreate = append(newSetsToCreate, gymSet)
			}
		}
		if err := gymSetRepo.ReplaceActivitySets(uint(id), newSetsToCreate); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to save workout: %v", err)
			return
		}

		log.Printf("Successfully saved %d sets for activity %d", len(newSetsToCreate), id)
		err = activityRepo.UpdateActivityStatus(uint(id), database.StatusActive)
		if err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to save workout: %v", err)
		}
		session := sessions.Default(ctx)
		session.Delete("active_workout_id")
		session.Save()

		ctx.Header("HX-Redirect", "/workouts/"+idStr)
		ctx.Status(http.StatusOK)
	}
}

func SaveDraftWorkoutHandler(gymSetRepo *database.GymSetRepo, activityRepo *database.ActivityRepo, exerciseRepo *database.ExerciseRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		idStr := ctx.Param("id")
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			ctx.String(http.StatusBadRequest, "Invalid activity ID")
			return
		}

		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to parse form: %v", err)
			return
		}

		form, err := bindWorkoutFormManually(ctx.Request.PostForm)
		if err != nil {
			ctx.String(http.StatusBadRequest, "Failed to bind form manually: %v", err)
			return
		}

		var newSetsToCreate []*database.GymSet
		currentSetNumber := 0
		for _, exercise := range form.Exercises {
			for _, set := range exercise.Sets {
				if exercise.ExerciseID == 0 {
					continue
				}
				currentSetNumber++
				gymSet := &database.GymSet{
					ActivityID:           uint(id),
					ExerciseDefinitionID: uint(exercise.ExerciseID),
					SetNumber:            currentSetNumber,
					Reps:                 set.Reps,
					WeightKG:             set.WeightKG,
				}
				newSetsToCreate = append(newSetsToCreate, gymSet)
			}
		}
		if err := gymSetRepo.ReplaceActivitySets(uint(id), newSetsToCreate); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to save workout: %v", err)
			return
		}

		log.Printf("Successfully saved %d sets for activity %d", len(newSetsToCreate), id)
		ctx.Status(http.StatusOK)
	}
}

var formKeyRegex = regexp.MustCompile(`exercises\[(\d+)\]\[(sets)\]\[(\d+)\]\[(\w+)\]|exercises\[(\d+)\]\[(\w+)\]`)

func bindWorkoutFormManually(data url.Values) (WorkoutForm, error) {
	var form WorkoutForm

	exercisesMap := make(map[int]*ExerciseData)
	for key, values := range data {
		println(key, values)
		if len(values) == 0 {
			continue
		}
		val := values[0]

		matches := formKeyRegex.FindStringSubmatch(key)
		if matches == nil {
			continue
		}

		if matches[5] != "" {
			exerciseIndex, _ := strconv.Atoi(matches[5])
			fieldName := matches[6]
			if _, ok := exercisesMap[exerciseIndex]; !ok {
				exercisesMap[exerciseIndex] = &ExerciseData{}
			}

			if fieldName == "exercise_id" {
				id, _ := strconv.ParseUint(val, 10, 64)
				exercisesMap[exerciseIndex].ExerciseID = id
			}
		}

		if matches[1] != "" {
			exerciseIndex, _ := strconv.Atoi(matches[1])
			setIndex, _ := strconv.Atoi(matches[3])
			fieldName := matches[4]

			if _, ok := exercisesMap[exerciseIndex]; !ok {
				exercisesMap[exerciseIndex] = &ExerciseData{}
			}
			for len(exercisesMap[exerciseIndex].Sets) <= setIndex {
				exercisesMap[exerciseIndex].Sets = append(exercisesMap[exerciseIndex].Sets, SetData{})
			}

			set := &exercisesMap[exerciseIndex].Sets[setIndex]
			switch fieldName {
			case "reps":
				reps, _ := strconv.Atoi(val)
				set.Reps = reps
			case "weight_kg":
				weight, _ := strconv.ParseFloat(val, 64)
				set.WeightKG = weight
			}
		}
	}

	// --- FIX IS HERE ---

	// 1. Get all the keys (exercise indexes) from the map
	var sortedKeys []int
	for k := range exercisesMap {
		sortedKeys = append(sortedKeys, k)
	}

	// 2. Sort the keys numerically to preserve the form's order
	sort.Ints(sortedKeys)

	// 3. Build the final slice by iterating over the sorted keys
	for _, key := range sortedKeys {
		// We check if the pointer is non-nil before dereferencing
		if exercisesMap[key] != nil {
			form.Exercises = append(form.Exercises, *exercisesMap[key])
		}
	}

	return form, nil
}

// AddExerciseModalHandler serves the modal content.
func AddExerciseModalHandler(exerciseRepo *database.ExerciseRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		allExercises, err := exerciseRepo.GetExerciseList()
		if err != nil {
			// Handle error
			c.String(http.StatusInternalServerError, "Could not fetch exercises")
			return
		}
		// You need the activity ID for the POST action inside the modal
		activityID := c.Param("id")

		c.HTML(http.StatusOK, "_add-exercise-modal.html", gin.H{
			"AllExercises": allExercises,
			"ActivityID":   activityID,
		})
	}
}

func AddExerciseToFormHandler(activityRepo *database.ActivityRepo, gymSetRepo *database.GymSetRepo, exerciseRepo *database.ExerciseRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		activityID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
		if err != nil {
			ctx.String(http.StatusBadRequest, "Invalid activity ID")
			return
		}

		// Get the selected exercise ID
		selectedExerciseID, err := strconv.ParseUint(ctx.PostForm("exercise_id"), 10, 64)
		if err != nil {
			ctx.String(http.StatusBadRequest, "Invalid exercise ID")
			return
		}

		// Parse the current form to get the existing exercises
		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to parse form: %v", err)
			return
		}

		form, err := bindWorkoutFormManually(ctx.Request.PostForm)
		if err != nil {
			// It's okay if this fails, might be the first exercise
			log.Printf("Could not bind form on add exercise to form: %v", err)
		}

		// Save the current state to the database
		var setsForDB []*database.GymSet
		currentSetNumber := 0
		if form.Exercises != nil {
			for _, exercise := range form.Exercises {
				if exercise.ExerciseID == 0 {
					continue
				}
				for _, set := range exercise.Sets {
					currentSetNumber++
					gymSet := &database.GymSet{
						ActivityID:           uint(activityID),
						ExerciseDefinitionID: uint(exercise.ExerciseID),
						SetNumber:            currentSetNumber,
						Reps:                 set.Reps,
						WeightKG:             set.WeightKG,
					}
					setsForDB = append(setsForDB, gymSet)
				}
			}
		}

		// Save the current state to preserve it
		if err := gymSetRepo.ReplaceActivitySets(uint(activityID), setsForDB); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to save current state: %v", err)
			return
		}

		// Get the new exercise index
		newExerciseIndex := len(form.Exercises)

		// Get exercise list for the template
		allExercises, err := exerciseRepo.GetExerciseList()
		if err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to get exercise list: %v", err)
			return
		}

		// Return just the new exercise block HTML
		ctx.HTML(http.StatusOK, "_exercise-block.html", gin.H{
			"Index":        newExerciseIndex,
			"ExerciseID":   selectedExerciseID,
			"Sets":         nil, // New exercise has no sets yet
			"AllExercises": allExercises,
			"ActivityID":   activityID,
		})
	}
}

func ExerciseInfoHandler(exerciseRepo *database.ExerciseRepo, gymSetRepo *database.GymSetRepo, userRepo *database.UserRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Get IDs
		sessionUserId := sessions.Default(ctx).Get("user").(uint)
		sessionUser, err := userRepo.GetUserById(uint64(sessionUserId))
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
		}
		exerciseID, err := strconv.ParseUint(ctx.Param("exerciseID"), 10, 64)
		if err != nil {
			ctx.String(http.StatusBadRequest, "Invalid Exercise ID")
			return
		}

		// Fetch data
		exercise, err := exerciseRepo.GetExerciseByID(uint(exerciseID))
		if err != nil {
			ctx.String(http.StatusNotFound, "Exercise not found")
			return
		}

		// --- ADDED ERROR CHECKING HERE ---
		history, err := gymSetRepo.GetExerciseHistoryForUser(sessionUser.ID, uint(exerciseID))
		if err != nil {
			// This will log the actual database error to your console
			log.Printf("Error fetching exercise history: %v", err)
			// Inform the user something went wrong
			ctx.String(http.StatusInternalServerError, "Could not fetch exercise history.")
			return
		}

		// --- Your grouping logic (this part is correct) ---
		type ActivityHistory struct {
			Activity database.Activity
			Sets     []*database.GymSet
		}
		var groupedHistory []ActivityHistory
		if len(history) > 0 {
			setsByActivityID := make(map[uint][]*database.GymSet)
			activitiesMap := make(map[uint]database.Activity)
			for _, set := range history {
				setsByActivityID[set.ActivityID] = append(setsByActivityID[set.ActivityID], set)
				if _, ok := activitiesMap[set.ActivityID]; !ok {
					activitiesMap[set.ActivityID] = set.Activity
				}
			}
			for id, activity := range activitiesMap {
				groupedHistory = append(groupedHistory, ActivityHistory{
					Activity: activity,
					Sets:     setsByActivityID[id],
				})
			}
		}
		// --- End of grouping logic ---

		activityID := ctx.Query("activityID")

		ctx.HTML(http.StatusOK, "_exercise-info.html", gin.H{
			"Exercise":       exercise,
			"GroupedHistory": groupedHistory,
			"ActivityID":     activityID, // Pass it to the template
		})
	}
}

// ExerciseListHandler serves ONLY the list of exercises for the modal.
func ExerciseListHandler(exerciseRepo *database.ExerciseRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		allExercises, _ := exerciseRepo.GetExerciseList()
		activityID := c.Param("id")

		c.HTML(http.StatusOK, "_exercise-list.html", gin.H{
			"AllExercises": allExercises,
			"ActivityID":   activityID,
		})
	}
}

func DeleteExerciseHandler(activityRepo *database.ActivityRepo, gymSetRepo *database.GymSetRepo, exerciseRepo *database.ExerciseRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		activityID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
		if err != nil {
			ctx.String(http.StatusBadRequest, "Invalid activity ID")
			return
		}

		activity, err := activityRepo.GetActivityByID(uint(activityID))
		if err != nil {
			ctx.String(http.StatusNotFound, "Workout not found")
			return
		}

		// Get the exercise index to delete
		exerciseIndexStr := ctx.PostForm("exercise_index")
		exerciseIndex, err := strconv.Atoi(exerciseIndexStr)
		if err != nil {
			ctx.String(http.StatusBadRequest, "Invalid exercise index")
			return
		}

		// Parse the form data from existing exercises
		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to parse form: %v", err)
			return
		}

		form, err := bindWorkoutFormManually(ctx.Request.PostForm)
		if err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to bind form: %v", err)
			return
		}

		println(exerciseIndex)
		for i, exercise := range form.Exercises {
			println(exercise.ExerciseID)
			println(i)
		}

		if exerciseIndex >= 0 && exerciseIndex < len(form.Exercises) {
			form.Exercises = append(form.Exercises[:exerciseIndex], form.Exercises[exerciseIndex+1:]...)
		}

		// Save the updated state
		var setsForDB []*database.GymSet
		currentSetNumber := 0
		if form.Exercises != nil {
			for _, exercise := range form.Exercises {
				if exercise.ExerciseID == 0 {
					continue
				}
				for _, set := range exercise.Sets {
					currentSetNumber++
					gymSet := &database.GymSet{
						ActivityID:           uint(activityID),
						ExerciseDefinitionID: uint(exercise.ExerciseID),
						SetNumber:            currentSetNumber,
						Reps:                 set.Reps,
						WeightKG:             set.WeightKG,
					}
					setsForDB = append(setsForDB, gymSet)
				}
			}
		}

		if err := gymSetRepo.ReplaceActivitySets(uint(activityID), setsForDB); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to save updated workout: %v", err)
			return
		}

		// Get updated populated exercises
		populatedExercises, err := gymSetRepo.GetPopulatedExercises(activity.ID)
		if err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to get populated exercises: %v", err)
			return
		}

		// Get exercise list for the template
		allExercises, err := exerciseRepo.GetExerciseList()
		if err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to get exercise list: %v", err)
			return
		}

		// Return the complete updated form
		ctx.HTML(http.StatusOK, "_form-contents.html", gin.H{
			"Activity":           activity,
			"PopulatedExercises": populatedExercises,
			"AllExercises":       allExercises,
		})
	}
}

func DiscardActivityHandler(activityRepo *database.ActivityRepo, gymSetRepo *database.GymSetRepo, exerciseRepo *database.ExerciseRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		activityID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
		if err != nil {
			ctx.String(http.StatusBadRequest, "Invalid activity ID")
			return
		}

		activity, err := activityRepo.GetActivityByID(uint(activityID))
		if err != nil {
			ctx.String(http.StatusNotFound, "Workout not found")
			return
		}

		if activity.Status == database.StatusActive {
			// For active workouts, discard changes by reverting to the original state
			// Get the original populated exercises from the database
			populatedExercises, err := gymSetRepo.GetPopulatedExercises(activity.ID)
			if err != nil {
				ctx.String(http.StatusInternalServerError, "Failed to get original workout: %v", err)
				return
			}

			// Get exercise list for the template
			allExercises, err := exerciseRepo.GetExerciseList()
			if err != nil {
				ctx.String(http.StatusInternalServerError, "Failed to get exercise list: %v", err)
				return
			}

			// Return the original form state
			ctx.HTML(http.StatusOK, "edit-workout.html", gin.H{
				"Activity":           activity,
				"PopulatedExercises": populatedExercises,
				"AllExercises":       allExercises,
				"NextExerciseIndex":  len(populatedExercises),
			})
		} else {
			// For draft workouts, delete the entire activity
			if err := activityRepo.DeleteActivity(uint(activityID)); err != nil {
				ctx.String(http.StatusInternalServerError, "Failed to delete workout: %v", err)
				return
			}

			// Clear the active workout from session
			session := sessions.Default(ctx)
			session.Delete("active_workout_id")
			err := session.Save()
			if err != nil {
				return
			}

			// Redirect to home page
			ctx.Header("HX-Redirect", "/user")
			ctx.Status(http.StatusOK)
		}
	}
}
