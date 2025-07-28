# Feature: Personal Best (PB) System

This feature will automatically track and notify users of new Personal Bests (PBs) they achieve during a workout. This includes records for both total volume and rep maxes for each exercise.

---

## 1. Database Schema

-   [ ] **Create the `personal_records` Table:** A new table is needed to flexibly store different types of records.

    **GORM Model (`database/models.go`):**
      ```go
      type PersonalRecord struct {
          gorm.Model
          UserID               uint
          ExerciseDefinitionID uint
  
          RecordType string  // e.g., "TOTAL_VOLUME", "1_REP_MAX", "5_REP_MAX"
          Value      float64 // The record value (volume in kg, or weight in kg)
          
          User               User
          ExerciseDefinition ExerciseDefinition
      }
      ```

-   [ ] **Add Model to `AutoMigrate`:** Ensure the new `&database.PersonalRecord{}` is added to your `db.AutoMigrate()` call to create the table.

---

## 2. Backend Logic

-   [ ] **Create the "PB Analyzer" Service:** This will be a new function or method that contains the core logic for checking records.

    **Location:** `workout/pb_analyzer.go` (suggested)
    **Function Signature:** `func AnalyzeWorkoutForPBs(activity *database.Activity) ([]string, error)`
    **Logic:**
    1.  The function receives a completed `Activity` with all its `GymExercises` and `Sets` preloaded.
    2.  It should group all sets by their `ExerciseDefinitionID`.
    3.  For each exercise in the workout, it will:
        -   Calculate the **total volume** (`reps × weight` for all sets) and compare it against the stored `TOTAL_VOLUME` PB for that exercise. If it's a new record, update the DB and add a descriptive string to a results slice.
        -   Loop through each **set** (e.g., 5 reps @ 100kg). Compare the weight against the stored `5_REP_MAX` PB. If it's a new record, update the DB and add a descriptive string to the results slice.
    4.  The function returns the slice of descriptive strings for all new PBs achieved.

-   [ ] **Integrate Analyzer into `FinishWorkoutHandler`:**

    **Location:** `workout/handlers.go`
    **Logic:**
    1.  After the workout is successfully saved, call the `AnalyzeWorkoutForPBs` function.
    2.  Check if the returned slice of new PBs is empty.
    3.  **If PBs were achieved:** Render a new modal partial (`_workout_summary_modal.html`), passing the list of PBs to it.
    4.  **If no PBs were achieved:** Perform the original `HX-Redirect` to the completed workout page.

---

## 3. Frontend / UI Flow

-   [ ] **Create the Post-Workout Summary Modal:** This modal will display the achievements.

    **File:** `templates/_workout_summary_modal.html`
    **Content:**
    -   A title, e.g., "New Personal Bests!"
    -   A `range` loop to display each string from the new PBs slice.
    -   A "Done" or "View Workout" button that is a simple `<a>` tag pointing to the completed workout's view page (e.g., `/workouts/{{ .ActivityID }}`).