package workout

import (
	"fitness/platform/database"
	"fmt"
	"gorm.io/gorm"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// CreateHandler handles the POST /workouts/new request
func CreateHandler(activityRepo *database.ActivityRepo, userRepo *database.UserRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		activeIDInterface := session.Get("active_workout_id")
		discard := ctx.Query("discard") == "true"

		// This confirmation flow is good, no changes needed here.
		if activeIDInterface != nil && !discard {
			activeID := activeIDInterface.(uint)
			returnURL := fmt.Sprintf("/workouts/%d/edit", activeID)
			discardAndStartNewUrl := "/workouts/new?discard=true"
			ctx.HTML(http.StatusOK, "_create_workout_confirm.html", gin.H{
				"DiscardURL": discardAndStartNewUrl,
				"ReturnURL":  returnURL,
			})
			return
		}

		// If a draft is being discarded, use the new robust delete method.
		if activeIDInterface != nil {
			if err := activityRepo.DeleteActivityAndChildren(activeIDInterface.(uint)); err != nil {
				ctx.String(http.StatusInternalServerError, "Failed to discard previous workout")
				return // Important to return here
			}
		}

		// The logic for creating the new activity is correct.
		sessionUserId := session.Get("user").(uint)
		sessionUser, _ := userRepo.GetUserById(uint64(sessionUserId))

		newActivity := &database.Activity{
			UserID:       sessionUser.ID,
			Type:         "GYM_WORKOUT",
			ActivityTime: time.Now(),
			Name:         "Gym Workout",
			Status:       database.StatusDraft,
		}
		activityRepo.CreateActivity(newActivity) // Assuming Create is a simple create method

		// The logic for setting the session and redirecting is also correct.
		session.Set("active_workout_id", newActivity.ID)
		session.Save()

		redirectURL := fmt.Sprintf("/workouts/%d/edit", newActivity.ID)
		ctx.Header("HX-Redirect", redirectURL)
		ctx.Status(http.StatusOK)
	}
}

func DeleteActivityHandler(activityRepo *database.ActivityRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {

		idStr := ctx.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to parse id")
		}
		err = activityRepo.DeleteActivityAndChildren(uint(id))
		if err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to delete activity")
		}
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

		ctx.HTML(http.StatusOK, "view-workout.html", gin.H{
			"Activity": activity,
			"User":     sessionUser,
		})
	}
}

func EditHandler(activityRepo *database.ActivityRepo, gymSetRepo *database.GymSetRepo, exerciseRepo *database.ExerciseRepo, userRepo *database.UserRepo) gin.HandlerFunc {
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

		ctx.HTML(http.StatusOK, "edit-workout.html", gin.H{
			"Activity":     activity,
			"AllExercises": allExercises,
			"User":         sessionUser,
		})
	}
}

// UpdateSetHandler handles updating a single set's reps and weight.
// Route: PUT /gym-set/:id
func UpdateSetHandler(gymSetRepo *database.GymSetRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		setID, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)

		reps, _ := strconv.Atoi(ctx.PostForm("reps"))
		weight, _ := strconv.ParseFloat(ctx.PostForm("weight_kg"), 64)

		newSet := database.GymSet{
			Model: gorm.Model{
				ID: uint(setID),
			},
			Reps:     reps,
			WeightKG: weight,
		}

		if err := gymSetRepo.UpdateSet(&newSet); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to update set")
			return
		}

		ctx.Status(http.StatusOK)
	}
}

// UpdateExerciseHandler handles changing the selected exercise definition.
// Route: PUT /gym-exercise/:id
func UpdateExerciseHandler(gymExerciseRepo *database.GymExerciseRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		exerciseID, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
		newDefinitionID, _ := strconv.ParseUint(ctx.PostForm("exercise_id"), 10, 64)

		exerciseToUpdate := &database.GymExercise{
			Model: gorm.Model{
				ID: uint(exerciseID),
			},
			ExerciseDefinitionID: uint(newDefinitionID),
		}

		if err := gymExerciseRepo.UpdateExercise(exerciseToUpdate); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to update exercise")
			return
		}

		ctx.Status(http.StatusOK)
	}
}

// AddExerciseToActivityHandler creates a new, blank GymExercise for an Activity.
func AddExerciseToActivityHandler(
	gymExerciseRepo *database.GymExerciseRepo,
	gymSetRepo *database.GymSetRepo,
	exerciseRepo *database.ExerciseRepo,
) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		activityID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
		if err != nil {
			ctx.String(http.StatusBadRequest, "Invalid activity ID")
			return
		}

		// 1. Get the current number of exercises to use as the sort order.
		currentExerciseCount, err := gymExerciseRepo.CountByActivityID(activityID)
		if err != nil {
			ctx.String(http.StatusInternalServerError, "Could not count exercises")
			return
		}

		// 2. Create the parent GymExercise record with the correct SortOrder.
		newGymExercise := &database.GymExercise{
			ActivityID: uint(activityID),
			SortNumber: int(currentExerciseCount) + 1,
		}

		if err := gymExerciseRepo.CreateGymExercise(newGymExercise); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to create new exercise")
			return
		}

		firstSet := &database.GymSet{
			GymExerciseID: newGymExercise.ID,
			SetNumber:     1,
		}
		if err := gymSetRepo.CreateGymSet(
			firstSet); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to create initial set")
			return
		}

		newGymExercise.Sets = []database.GymSet{*firstSet}

		allExercises, _ := exerciseRepo.GetExerciseList()

		ctx.HTML(http.StatusOK, "_exercise-block.html", gin.H{
			"Index":        currentExerciseCount,
			"GymExercise":  newGymExercise,
			"AllExercises": allExercises,
			"ActivityID":   activityID,
		})
	}
}

// AddSetToExerciseHandler creates a new, blank GymSet for a GymExercise.
func AddSetToExerciseHandler(gymSetRepo *database.GymSetRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		gymExerciseID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
		if err != nil {
			ctx.String(http.StatusBadRequest, "Invalid exercise ID")
			return
		}

		// 1. Determine the next set number.
		setCount, err := gymSetRepo.CountByExerciseID(uint64(uint(gymExerciseID)))
		if err != nil {
			ctx.String(http.StatusInternalServerError, "Could not count sets")
			return
		}
		nextSetNumber := setCount + 1

		// 2. Create the new GymSet record.
		newSet := &database.GymSet{
			GymExerciseID: uint(gymExerciseID),
			SetNumber:     int(nextSetNumber),
		}

		if err := gymSetRepo.CreateGymSet(newSet); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to create set")
			return
		}

		// 3. Return just the new set row HTML.
		// HTMX will append this to the container of sets.
		ctx.HTML(http.StatusOK, "_exercise-set.html", gin.H{
			"Set": newSet,
		})
	}
}

// AddExerciseModalHandler serves the modal container.
// The modal itself then loads its content via hx-get.
// Route: GET /ui/add-exercise-modal/:id
func AddExerciseModalHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		activityID := c.Param("id")
		c.HTML(http.StatusOK, "_add-exercise-modal.html", gin.H{
			"ActivityID": activityID,
		})
	}
}

// ExerciseListHandler serves the list of all exercises inside the modal.
// Route: GET /ui/exercise-list/:id
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

// ExerciseInfoHandler shows details and history for a single exercise.
// Route: GET /exercise-info/:exerciseID
func ExerciseInfoHandler(
	exerciseRepo *database.ExerciseRepo,
	gymSetRepo *database.GymSetRepo,
	userRepo *database.UserRepo,
) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		sessionUserId := sessions.Default(ctx).Get("user").(uint)
		sessionUser, _ := userRepo.GetUserById(uint64(sessionUserId))
		exerciseID, _ := strconv.ParseUint(ctx.Param("exerciseID"), 10, 64)
		activityID := ctx.Query("activityID")

		exercise, _ := exerciseRepo.GetExerciseByID(uint(exerciseID))
		history, _ := gymSetRepo.GetExerciseHistoryForUser(sessionUser.ID, uint(exerciseID))

		// Grouping logic for history can go here...

		ctx.HTML(http.StatusOK, "_exercise-info.html", gin.H{
			"Exercise":       exercise,
			"GroupedHistory": history, // Pass your grouped history here
			"ActivityID":     activityID,
		})
	}
}

// AddExerciseToFormHandler creates the new GymExercise and its first set.
func AddExerciseToFormHandler(
	gymExerciseRepo *database.GymExerciseRepo,
	gymSetRepo *database.GymSetRepo,
	exerciseRepo *database.ExerciseRepo,
) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		activityID, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)
		exerciseDefinitionID_uint64, _ := strconv.ParseUint(ctx.PostForm("exercise_id"), 10, 64)

		defID := uint(exerciseDefinitionID_uint64)

		currentExerciseCount, _ := gymExerciseRepo.CountByActivityID(activityID)

		newGymExercise := &database.GymExercise{
			ActivityID:           uint(activityID),
			ExerciseDefinitionID: defID,
			SortNumber:           int(currentExerciseCount),
		}
		gymExerciseRepo.CreateGymExercise(newGymExercise)

		firstSet := &database.GymSet{
			GymExerciseID: newGymExercise.ID,
			SetNumber:     1,
		}
		gymSetRepo.CreateGymSet(firstSet)

		definition, err := exerciseRepo.GetExerciseByID(defID)
		if err != nil {
			ctx.String(http.StatusInternalServerError, "Could not find exercise definition")
			return
		}

		newGymExercise.ExerciseDefinition = *definition
		newGymExercise.Sets = []database.GymSet{*firstSet}
		allExercises, _ := exerciseRepo.GetExerciseList()

		ctx.HTML(http.StatusOK, "_exercise-block.html", gin.H{
			"Index":        currentExerciseCount,
			"GymExercise":  newGymExercise,
			"AllExercises": allExercises,
			"ActivityID":   activityID,
		})
	}
}

// DeleteSetHandler handles deleting a single set.
// Route: DELETE /gym-set/:id
func DeleteSetHandler(gymSetRepo *database.GymSetRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		setID, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)

		if err := gymSetRepo.DeleteSet(uint(setID)); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to delete set")
			return
		}

		// Return 200 OK with no content. HTMX will remove the element.
		ctx.Status(http.StatusOK)
	}
}

// DeleteExerciseHandler handles deleting an exercise and all its sets.
// Route: DELETE /gym-exercise/:id
func DeleteExerciseHandler(gymExerciseRepo *database.GymExerciseRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		exerciseID, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)

		if err := gymExerciseRepo.DeleteExercise(uint(exerciseID)); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to delete exercise")
			return
		}

		ctx.Status(http.StatusOK)
	}
}

// CreateEditDraftHandler makes a draft copy of an existing active workout.
// Route: POST /workouts/:id/create-edit-draft
func CreateEditDraftHandler(activityRepo *database.ActivityRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		originalID, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)

		// Create a deep copy and get the new draft's ID
		draftID, err := activityRepo.CreateDraftCopy(uint(originalID))
		if err != nil {
			ctx.String(http.StatusInternalServerError, "Could not create draft")
			return
		}

		session := sessions.Default(ctx)
		session.Set("active_workout_id", draftID)
		if err := session.Save(); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to save session")
			return
		}

		// Redirect to the edit page for the new draft
		ctx.Redirect(http.StatusFound, fmt.Sprintf("/workouts/%d/edit", draftID))
	}
}

// FinishWorkoutHandler promotes a draft to active, replacing the original if it exists.
// Route: POST /activity/:id/finish
func FinishWorkoutHandler(activityRepo *database.ActivityRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		draftID, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)

		// This new repo method contains the logic to promote the draft
		finalID, err := activityRepo.FinalizeDraft(uint(draftID))
		if err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to finalize workout")
			return
		}

		session := sessions.Default(ctx)
		session.Delete("active_workout_id")
		if err := session.Save(); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to save session")
			return
		}

		// Redirect to the newly active workout's view page
		ctx.Header("HX-Redirect", fmt.Sprintf("/workouts/%d", finalID))
		ctx.Status(http.StatusOK)
	}
}

// DiscardWorkoutHandler deletes a draft and redirects appropriately.
// Route: POST /activity/:id/discard
func DiscardWorkoutHandler(activityRepo *database.ActivityRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		draftID, _ := strconv.ParseUint(ctx.Param("id"), 10, 64)

		draft, err := activityRepo.GetActivityByID(uint(draftID)) // Use your existing GetActivityByID
		if err != nil || draft == nil {
			ctx.Redirect(http.StatusFound, "/user") // Redirect home if draft not found
			return
		}

		// Delete the draft record
		activityRepo.DeleteActivity(uint(draftID)) // Use your existing DeleteActivity

		// If this draft was a copy of an original, redirect to the original.
		// Otherwise, it was a new workout, so redirect home.
		redirectURL := "/user"
		if draft.OriginalActivityID != nil {
			redirectURL = fmt.Sprintf("/workouts/%d", *draft.OriginalActivityID)
		}

		session := sessions.Default(ctx)
		session.Delete("active_workout_id")
		if err := session.Save(); err != nil {
			ctx.String(http.StatusInternalServerError, "Failed to save session")
			return
		}

		ctx.Header("HX-Redirect", redirectURL)
		ctx.Status(http.StatusOK)
	}
}

func GetActivityNameHandler(activityRepo *database.ActivityRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		activityIDParam := ctx.Param("id")
		activityID, err := strconv.ParseUint(activityIDParam, 10, 64)
		if err != nil {
			ctx.String(http.StatusBadRequest, "Invalid activity ID")
			return
		}

		var editMode bool
		editQuery := ctx.Query("edit")
		if editQuery != "" {
			parsedEditMode, err := strconv.ParseBool(editQuery)
			if err != nil {
				ctx.String(http.StatusBadRequest, "Invalid value for 'edit' query parameter")
				return
			}
			editMode = parsedEditMode
		}

		activity, err := activityRepo.GetActivityByID(uint(activityID))
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				ctx.String(http.StatusNotFound, "Activity not found")
				return
			}
			ctx.String(http.StatusInternalServerError, "Failed to get activity")
			return
		}

		ctx.HTML(http.StatusOK, "_activity_name.html", gin.H{
			"Activity": activity,
			"EditMode": editMode,
		})
	}
}

func UpdateActivityNameHandler(activityRepo *database.ActivityRepo) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		activityIDParam := ctx.Param("id")
		activityID, err := strconv.ParseUint(activityIDParam, 10, 64)
		if err != nil {
			ctx.String(http.StatusBadRequest, "Invalid activity ID")
			return
		}

		// FIX: Added input validation
		newName := ctx.PostForm("name")
		if strings.TrimSpace(newName) == "" {
			ctx.String(http.StatusBadRequest, "Activity name cannot be empty.")
			return
		}

		updatedActivity, err := activityRepo.UpdateActivityName(uint(activityID), newName)
		if err != nil {
			// Check if the record didn't exist to begin with
			if err == gorm.ErrRecordNotFound {
				ctx.String(http.StatusNotFound, "Activity not found")
				return
			}
			// Handle other potential database errors
			ctx.String(http.StatusInternalServerError, "Failed to update activity name")
			return
		}

		// Now we render the display view of the component with the updated data
		ctx.HTML(http.StatusOK, "_activity_name.html", gin.H{
			"Activity": updatedActivity, // Use the returned object
			"EditMode": false,
		})
	}
}

//type SetData struct {
//	Reps     int     `form:"reps"`
//	WeightKG float64 `form:"weight_kg"`
//}
//
//type ExerciseData struct {
//	ExerciseID uint64    `form:"exercise_id"`
//	Sets       []SetData `form:"sets"`
//}
//
//type WorkoutForm struct {
//	Exercises []ExerciseData `form:"exercises"`
//}
//
//func SaveWorkoutHandler(gymSetRepo *database.GymSetRepo, activityRepo *database.ActivityRepo) gin.HandlerFunc {
//	return func(ctx *gin.Context) {
//		idStr := ctx.Param("id")
//		id, err := strconv.ParseUint(idStr, 10, 64)
//		if err != nil {
//			ctx.String(http.StatusBadRequest, "Invalid activity ID")
//			return
//		}
//
//		if err := ctx.Request.ParseForm(); err != nil {
//			ctx.String(http.StatusInternalServerError, "Failed to parse form: %v", err)
//			return
//		}
//
//		form, err := bindWorkoutFormManually(ctx.Request.PostForm)
//		if err != nil {
//			ctx.String(http.StatusBadRequest, "Failed to bind form manually: %v", err)
//			return
//		}
//
//		var newSetsToCreate []*database.GymSet
//		currentSetNumber := 0
//		for _, exercise := range form.Exercises {
//			for _, set := range exercise.Sets {
//				currentSetNumber++
//				gymSet := &database.GymSet{
//					ActivityID:           uint(id),
//					ExerciseDefinitionID: uint(exercise.ExerciseID),
//					SetNumber:            currentSetNumber,
//					Reps:                 set.Reps,
//					WeightKG:             set.WeightKG,
//				}
//				newSetsToCreate = append(newSetsToCreate, gymSet)
//			}
//		}
//		if err := gymSetRepo.ReplaceActivitySets(uint(id), newSetsToCreate); err != nil {
//			ctx.String(http.StatusInternalServerError, "Failed to save workout: %v", err)
//			return
//		}
//
//		log.Printf("Successfully saved %d sets for activity %d", len(newSetsToCreate), id)
//		err = activityRepo.UpdateActivityStatus(uint(id), database.StatusActive)
//		if err != nil {
//			ctx.String(http.StatusInternalServerError, "Failed to save workout: %v", err)
//		}
//		session := sessions.Default(ctx)
//		session.Delete("active_workout_id")
//		session.Save()
//
//		ctx.Header("HX-Redirect", "/workouts/"+idStr)
//		ctx.Status(http.StatusOK)
//	}
//}
//
//func SaveDraftWorkoutHandler(gymSetRepo *database.GymSetRepo, activityRepo *database.ActivityRepo, exerciseRepo *database.ExerciseRepo) gin.HandlerFunc {
//	return func(ctx *gin.Context) {
//		idStr := ctx.Param("id")
//		id, err := strconv.ParseUint(idStr, 10, 64)
//		if err != nil {
//			ctx.String(http.StatusBadRequest, "Invalid activity ID")
//			return
//		}
//
//		if err := ctx.Request.ParseForm(); err != nil {
//			ctx.String(http.StatusInternalServerError, "Failed to parse form: %v", err)
//			return
//		}
//
//		form, err := bindWorkoutFormManually(ctx.Request.PostForm)
//		if err != nil {
//			ctx.String(http.StatusBadRequest, "Failed to bind form manually: %v", err)
//			return
//		}
//
//		var newSetsToCreate []*database.GymSet
//		currentSetNumber := 0
//		for _, exercise := range form.Exercises {
//			for _, set := range exercise.Sets {
//				if exercise.ExerciseID == 0 {
//					continue
//				}
//				currentSetNumber++
//				gymSet := &database.GymSet{
//					ActivityID:           uint(id),
//					ExerciseDefinitionID: uint(exercise.ExerciseID),
//					SetNumber:            currentSetNumber,
//					Reps:                 set.Reps,
//					WeightKG:             set.WeightKG,
//				}
//				newSetsToCreate = append(newSetsToCreate, gymSet)
//			}
//		}
//		if err := gymSetRepo.ReplaceActivitySets(uint(id), newSetsToCreate); err != nil {
//			ctx.String(http.StatusInternalServerError, "Failed to save workout: %v", err)
//			return
//		}
//
//		log.Printf("Successfully saved %d sets for activity %d", len(newSetsToCreate), id)
//		ctx.Status(http.StatusOK)
//	}
//}
//
//var formKeyRegex = regexp.MustCompile(`exercises\[(\d+)\]\[(sets)\]\[(\d+)\]\[(\w+)\]|exercises\[(\d+)\]\[(\w+)\]`)
//
//func bindWorkoutFormManually(data url.Values) (WorkoutForm, error) {
//	var form WorkoutForm
//
//	exercisesMap := make(map[int]*ExerciseData)
//	for key, values := range data {
//		println(key, values)
//		if len(values) == 0 {
//			continue
//		}
//		val := values[0]
//
//		matches := formKeyRegex.FindStringSubmatch(key)
//		if matches == nil {
//			continue
//		}
//
//		if matches[5] != "" {
//			exerciseIndex, _ := strconv.Atoi(matches[5])
//			fieldName := matches[6]
//			if _, ok := exercisesMap[exerciseIndex]; !ok {
//				exercisesMap[exerciseIndex] = &ExerciseData{}
//			}
//
//			if fieldName == "exercise_id" {
//				id, _ := strconv.ParseUint(val, 10, 64)
//				exercisesMap[exerciseIndex].ExerciseID = id
//			}
//		}
//
//		if matches[1] != "" {
//			exerciseIndex, _ := strconv.Atoi(matches[1])
//			setIndex, _ := strconv.Atoi(matches[3])
//			fieldName := matches[4]
//
//			if _, ok := exercisesMap[exerciseIndex]; !ok {
//				exercisesMap[exerciseIndex] = &ExerciseData{}
//			}
//			for len(exercisesMap[exerciseIndex].Sets) <= setIndex {
//				exercisesMap[exerciseIndex].Sets = append(exercisesMap[exerciseIndex].Sets, SetData{})
//			}
//
//			set := &exercisesMap[exerciseIndex].Sets[setIndex]
//			switch fieldName {
//			case "reps":
//				reps, _ := strconv.Atoi(val)
//				set.Reps = reps
//			case "weight_kg":
//				weight, _ := strconv.ParseFloat(val, 64)
//				set.WeightKG = weight
//			}
//		}
//	}
//
//	// --- FIX IS HERE ---
//
//	// 1. Get all the keys (exercise indexes) from the map
//	var sortedKeys []int
//	for k := range exercisesMap {
//		sortedKeys = append(sortedKeys, k)
//	}
//
//	// 2. Sort the keys numerically to preserve the form's order
//	sort.Ints(sortedKeys)
//
//	// 3. Build the final slice by iterating over the sorted keys
//	for _, key := range sortedKeys {
//		// We check if the pointer is non-nil before dereferencing
//		if exercisesMap[key] != nil {
//			form.Exercises = append(form.Exercises, *exercisesMap[key])
//		}
//	}
//
//	return form, nil
//}
//
//// AddExerciseModalHandler serves the modal content.
//func AddExerciseModalHandler(exerciseRepo *database.ExerciseRepo) gin.HandlerFunc {
//	return func(c *gin.Context) {
//		allExercises, err := exerciseRepo.GetExerciseList()
//		if err != nil {
//			// Handle error
//			c.String(http.StatusInternalServerError, "Could not fetch exercises")
//			return
//		}
//		// You need the activity ID for the POST action inside the modal
//		activityID := c.Param("id")
//
//		c.HTML(http.StatusOK, "_add-exercise-modal.html", gin.H{
//			"AllExercises": allExercises,
//			"ActivityID":   activityID,
//		})
//	}
//}
//
//func AddExerciseToFormHandler(activityRepo *database.ActivityRepo, gymSetRepo *database.GymSetRepo, exerciseRepo *database.ExerciseRepo) gin.HandlerFunc {
//	return func(ctx *gin.Context) {
//		activityID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
//		if err != nil {
//			ctx.String(http.StatusBadRequest, "Invalid activity ID")
//			return
//		}
//
//		// Get the selected exercise ID
//		selectedExerciseID, err := strconv.ParseUint(ctx.PostForm("exercise_id"), 10, 64)
//		if err != nil {
//			ctx.String(http.StatusBadRequest, "Invalid exercise ID")
//			return
//		}
//
//		// Parse the current form to get the existing exercises
//		if err := ctx.Request.ParseForm(); err != nil {
//			ctx.String(http.StatusInternalServerError, "Failed to parse form: %v", err)
//			return
//		}
//
//		form, err := bindWorkoutFormManually(ctx.Request.PostForm)
//		if err != nil {
//			// It's okay if this fails, might be the first exercise
//			log.Printf("Could not bind form on add exercise to form: %v", err)
//		}
//
//		// Save the current state to the database
//		var setsForDB []*database.GymSet
//		currentSetNumber := 0
//		if form.Exercises != nil {
//			for _, exercise := range form.Exercises {
//				if exercise.ExerciseID == 0 {
//					continue
//				}
//				for _, set := range exercise.Sets {
//					currentSetNumber++
//					gymSet := &database.GymSet{
//						ActivityID:           uint(activityID),
//						ExerciseDefinitionID: uint(exercise.ExerciseID),
//						SetNumber:            currentSetNumber,
//						Reps:                 set.Reps,
//						WeightKG:             set.WeightKG,
//					}
//					setsForDB = append(setsForDB, gymSet)
//				}
//			}
//		}
//
//		// Save the current state to preserve it
//		if err := gymSetRepo.ReplaceActivitySets(uint(activityID), setsForDB); err != nil {
//			ctx.String(http.StatusInternalServerError, "Failed to save current state: %v", err)
//			return
//		}
//
//		// Get the new exercise index
//		newExerciseIndex := len(form.Exercises)
//
//		// Get exercise list for the template
//		allExercises, err := exerciseRepo.GetExerciseList()
//		if err != nil {
//			ctx.String(http.StatusInternalServerError, "Failed to get exercise list: %v", err)
//			return
//		}
//
//		// Return just the new exercise block HTML
//		ctx.HTML(http.StatusOK, "_exercise-block.html", gin.H{
//			"Index":        newExerciseIndex,
//			"ExerciseID":   selectedExerciseID,
//			"Sets":         nil, // New exercise has no sets yet
//			"AllExercises": allExercises,
//			"ActivityID":   activityID,
//		})
//	}
//}
//
//func ExerciseInfoHandler(exerciseRepo *database.ExerciseRepo, gymSetRepo *database.GymSetRepo, userRepo *database.UserRepo) gin.HandlerFunc {
//	return func(ctx *gin.Context) {
//		// Get IDs
//		sessionUserId := sessions.Default(ctx).Get("user").(uint)
//		sessionUser, err := userRepo.GetUserById(uint64(sessionUserId))
//		if err != nil {
//			ctx.String(http.StatusInternalServerError, err.Error())
//		}
//		exerciseID, err := strconv.ParseUint(ctx.Param("exerciseID"), 10, 64)
//		if err != nil {
//			ctx.String(http.StatusBadRequest, "Invalid Exercise ID")
//			return
//		}
//
//		// Fetch data
//		exercise, err := exerciseRepo.GetExerciseByID(uint(exerciseID))
//		if err != nil {
//			ctx.String(http.StatusNotFound, "Exercise not found")
//			return
//		}
//
//		// --- ADDED ERROR CHECKING HERE ---
//		history, err := gymSetRepo.GetExerciseHistoryForUser(sessionUser.ID, uint(exerciseID))
//		if err != nil {
//			// This will log the actual database error to your console
//			log.Printf("Error fetching exercise history: %v", err)
//			// Inform the user something went wrong
//			ctx.String(http.StatusInternalServerError, "Could not fetch exercise history.")
//			return
//		}
//
//		// --- Your grouping logic (this part is correct) ---
//		type ActivityHistory struct {
//			Activity database.Activity
//			Sets     []*database.GymSet
//		}
//		var groupedHistory []ActivityHistory
//		if len(history) > 0 {
//			setsByActivityID := make(map[uint][]*database.GymSet)
//			activitiesMap := make(map[uint]database.Activity)
//			for _, set := range history {
//				setsByActivityID[set.ActivityID] = append(setsByActivityID[set.ActivityID], set)
//				if _, ok := activitiesMap[set.ActivityID]; !ok {
//					activitiesMap[set.ActivityID] = set.Activity
//				}
//			}
//			for id, activity := range activitiesMap {
//				groupedHistory = append(groupedHistory, ActivityHistory{
//					Activity: activity,
//					Sets:     setsByActivityID[id],
//				})
//			}
//		}
//		// --- End of grouping logic ---
//
//		activityID := ctx.Query("activityID")
//
//		ctx.HTML(http.StatusOK, "_exercise-info.html", gin.H{
//			"Exercise":       exercise,
//			"GroupedHistory": groupedHistory,
//			"ActivityID":     activityID, // Pass it to the template
//		})
//	}
//}
//
//// ExerciseListHandler serves ONLY the list of exercises for the modal.
//func ExerciseListHandler(exerciseRepo *database.ExerciseRepo) gin.HandlerFunc {
//	return func(c *gin.Context) {
//		allExercises, _ := exerciseRepo.GetExerciseList()
//		activityID := c.Param("id")
//
//		c.HTML(http.StatusOK, "_exercise-list.html", gin.H{
//			"AllExercises": allExercises,
//			"ActivityID":   activityID,
//		})
//	}
//}
//
//func DeleteExerciseHandler(activityRepo *database.ActivityRepo, gymSetRepo *database.GymSetRepo, exerciseRepo *database.ExerciseRepo) gin.HandlerFunc {
//	return func(ctx *gin.Context) {
//		activityID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
//		if err != nil {
//			ctx.String(http.StatusBadRequest, "Invalid activity ID")
//			return
//		}
//
//		activity, err := activityRepo.GetActivityByID(uint(activityID))
//		if err != nil {
//			ctx.String(http.StatusNotFound, "Workout not found")
//			return
//		}
//
//		// Get the exercise index to delete
//		exerciseIndexStr := ctx.PostForm("exercise_index")
//		exerciseIndex, err := strconv.Atoi(exerciseIndexStr)
//		if err != nil {
//			ctx.String(http.StatusBadRequest, "Invalid exercise index")
//			return
//		}
//
//		// Parse the form data from existing exercises
//		if err := ctx.Request.ParseForm(); err != nil {
//			ctx.String(http.StatusInternalServerError, "Failed to parse form: %v", err)
//			return
//		}
//
//		form, err := bindWorkoutFormManually(ctx.Request.PostForm)
//		if err != nil {
//			ctx.String(http.StatusInternalServerError, "Failed to bind form: %v", err)
//			return
//		}
//
//		println(exerciseIndex)
//		for i, exercise := range form.Exercises {
//			println(exercise.ExerciseID)
//			println(i)
//		}
//
//		if exerciseIndex >= 0 && exerciseIndex < len(form.Exercises) {
//			form.Exercises = append(form.Exercises[:exerciseIndex], form.Exercises[exerciseIndex+1:]...)
//		}
//
//		// Save the updated state
//		var setsForDB []*database.GymSet
//		currentSetNumber := 0
//		if form.Exercises != nil {
//			for _, exercise := range form.Exercises {
//				if exercise.ExerciseID == 0 {
//					continue
//				}
//				for _, set := range exercise.Sets {
//					currentSetNumber++
//					gymSet := &database.GymSet{
//						ActivityID:           uint(activityID),
//						ExerciseDefinitionID: uint(exercise.ExerciseID),
//						SetNumber:            currentSetNumber,
//						Reps:                 set.Reps,
//						WeightKG:             set.WeightKG,
//					}
//					setsForDB = append(setsForDB, gymSet)
//				}
//			}
//		}
//
//		if err := gymSetRepo.ReplaceActivitySets(uint(activityID), setsForDB); err != nil {
//			ctx.String(http.StatusInternalServerError, "Failed to save updated workout: %v", err)
//			return
//		}
//
//		// Get updated populated exercises
//		populatedExercises, err := gymSetRepo.GetPopulatedExercises(activity.ID)
//		if err != nil {
//			ctx.String(http.StatusInternalServerError, "Failed to get populated exercises: %v", err)
//			return
//		}
//
//		// Get exercise list for the template
//		allExercises, err := exerciseRepo.GetExerciseList()
//		if err != nil {
//			ctx.String(http.StatusInternalServerError, "Failed to get exercise list: %v", err)
//			return
//		}
//
//		// Return the complete updated form
//		ctx.HTML(http.StatusOK, "_form-contents.html", gin.H{
//			"Activity":           activity,
//			"PopulatedExercises": populatedExercises,
//			"AllExercises":       allExercises,
//		})
//	}
//}
//
//func DiscardActivityHandler(activityRepo *database.ActivityRepo, gymSetRepo *database.GymSetRepo, exerciseRepo *database.ExerciseRepo) gin.HandlerFunc {
//	return func(ctx *gin.Context) {
//		activityID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
//		if err != nil {
//			ctx.String(http.StatusBadRequest, "Invalid activity ID")
//			return
//		}
//
//		activity, err := activityRepo.GetActivityByID(uint(activityID))
//		if err != nil {
//			ctx.String(http.StatusNotFound, "Workout not found")
//			return
//		}
//
//		if activity.Status == database.StatusActive {
//			// For active workouts, discard changes by reverting to the original state
//			// Get the original populated exercises from the database
//			populatedExercises, err := gymSetRepo.GetPopulatedExercises(activity.ID)
//			if err != nil {
//				ctx.String(http.StatusInternalServerError, "Failed to get original workout: %v", err)
//				return
//			}
//
//			// Get exercise list for the template
//			allExercises, err := exerciseRepo.GetExerciseList()
//			if err != nil {
//				ctx.String(http.StatusInternalServerError, "Failed to get exercise list: %v", err)
//				return
//			}
//
//			// Return the original form state
//			ctx.HTML(http.StatusOK, "edit-workout.html", gin.H{
//				"Activity":           activity,
//				"PopulatedExercises": populatedExercises,
//				"AllExercises":       allExercises,
//				"NextExerciseIndex":  len(populatedExercises),
//			})
//		} else {
//			// For draft workouts, delete the entire activity
//			if err := activityRepo.DeleteActivity(uint(activityID)); err != nil {
//				ctx.String(http.StatusInternalServerError, "Failed to delete workout: %v", err)
//				return
//			}
//
//			// Clear the active workout from session
//			session := sessions.Default(ctx)
//			session.Delete("active_workout_id")
//			err := session.Save()
//			if err != nil {
//				return
//			}
//
//			// Redirect to home page
//			ctx.Header("HX-Redirect", "/user")
//			ctx.Status(http.StatusOK)
//		}
//	}
//}
