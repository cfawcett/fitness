package database

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ActivityRepo struct {
	DB *gorm.DB
}

// NewActivityRepo creates a new ActivityRepo
func NewActivityRepo(db *gorm.DB) *ActivityRepo {
	return &ActivityRepo{DB: db}
}

// CreateActivity adds a new activity to the database
func (r *ActivityRepo) CreateActivity(activity *Activity) error {
	result := r.DB.Create(activity)
	return result.Error
}

// GetActivitiesByUserID returns a list of activities for a given user
func (r *ActivityRepo) GetActivitiesByUserID(userID uint) ([]*Activity, error) {
	var activities []*Activity
	result := r.DB.Where("user_id = ?", userID).Order("activity_time desc").Find(&activities)
	if result.Error != nil {
		return nil, result.Error
	}
	return activities, nil
}

// GetActivityByID returns the activity based on its database id
func (r *ActivityRepo) GetActivityByID(id uint) (*Activity, error) {
	var activity Activity

	err := r.DB.
		// Preload GymExercises and all their fields
		Preload("GymExercises", func(db *gorm.DB) *gorm.DB {
			// Also ensure they are sorted by the order you set
			return db.Order("gym_exercises.sort_number ASC")
		}).
		// Preload the sets for each of those exercises
		Preload("GymExercises.Sets", func(db *gorm.DB) *gorm.DB {
			return db.Order("gym_sets.set_number ASC")
		}).
		// Preload the definition for each exercise to get its name
		Preload("GymExercises.ExerciseDefinition").
		// Find the top-level activity
		First(&activity, id).Error

	return &activity, err
}

// UpdateActivityStatus updates the status of a specific activity.
func (r *ActivityRepo) UpdateActivityStatus(activityID uint, status ExerciseStatus) error {
	err := r.DB.Model(&Activity{}).Where("id = ?", activityID).Update("status", status).Error
	return err
}

// UpdateActivityName updates the name of a specific activity and returns the updated record.
func (r *ActivityRepo) UpdateActivityName(activityID uint, name string) (*Activity, error) {
	var activity Activity

	// Use Clauses(clause.Returning{}) to update and return the data in one query.

	// This is more efficient than a separate UPDATE and SELECT.
	err := r.DB.Model(&activity).
		Clauses(clause.Returning{}).
		Where("id = ?", activityID).
		Update("name", name).Error

	if err != nil {
		return nil, err
	}

	// If the update affected 0 rows (e.g., wrong ID), the returned activity.ID will be 0.
	// We check for this and return a standard "not found" error.
	if activity.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return &activity, nil
}

// UpdateActivityNotes updates the notes of a specific activity and returns the updated record.
func (r *ActivityRepo) UpdateActivityNotes(activityID uint, notes string) (*Activity, error) {
	var activity Activity

	err := r.DB.Model(&activity).
		Clauses(clause.Returning{}).
		Where("id = ?", activityID).
		Update("notes", notes).Error

	if err != nil {
		return nil, err
	}

	if activity.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return &activity, nil
}

// DeleteActivity deletes an activity and its associated gym sets
func (r *ActivityRepo) DeleteActivity(activityID uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		// First delete all gym sets associated with this activity
		if err := tx.Where("activity_id = ?", activityID).Delete(&GymSet{}).Error; err != nil {
			return err
		}

		// Then delete the activity itself
		if err := tx.Delete(&Activity{}, activityID).Error; err != nil {
			return err
		}

		return nil
	})
}

// CreateDraftCopy performs a deep copy of an activity and all its children,
// creating a new draft version. It correctly remaps superset links.
func (r *ActivityRepo) CreateDraftCopy(originalID uint) (uint, error) {
	var draftID uint

	// Use a transaction to ensure all or nothing is committed
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		// 1. Load the original activity with all its relationships
		var originalActivity Activity
		if err := tx.Preload("GymExercises.Sets").First(&originalActivity, originalID).Error; err != nil {
			return err
		}

		// 2. Create the new draft Activity shell
		draftActivity := originalActivity
		draftActivity.ID = 0 // Setting ID to 0 tells GORM to create a new record
		draftActivity.Status = StatusDraft
		draftActivity.OriginalActivityID = &originalActivity.ID
		draftActivity.GymExercises = nil // Clear associations to avoid GORM conflicts

		if err := tx.Create(&draftActivity).Error; err != nil {
			return err
		}
		draftID = draftActivity.ID

		// 3. Create a map to track old exercise IDs to their new IDs
		oldToNewExerciseIDMap := make(map[uint]uint)

		// 4. First Pass: Copy all exercises and their sets.
		// We do this first to generate all the new IDs.
		for _, originalExercise := range originalActivity.GymExercises {
			newExercise := originalExercise
			newExercise.ID = 0
			newExercise.ActivityID = draftActivity.ID
			newExercise.SupersetWithID = nil // IMPORTANT: Keep this nil for now
			newExercise.Sets = nil

			// Copy all sets for this exercise
			for _, originalSet := range originalExercise.Sets {
				newSet := originalSet
				newSet.ID = 0
				newExercise.Sets = append(newExercise.Sets, newSet)
			}

			// Create the new exercise and its sets
			if err := tx.Create(&newExercise).Error; err != nil {
				return err
			}

			// Store the mapping from the old ID to the new one
			oldToNewExerciseIDMap[originalExercise.ID] = newExercise.ID
		}

		// 5. Second Pass: Update the new exercises with the correct superset links.
		// Now we have all the new IDs in our map.
		for _, originalExercise := range originalActivity.GymExercises {
			// If the original had a superset link...
			if originalExercise.SupersetWithID != nil {
				// Find the new ID for the original exercise
				newExerciseID := oldToNewExerciseIDMap[originalExercise.ID]
				// Find the new ID for its parent exercise
				newParentID := oldToNewExerciseIDMap[*originalExercise.SupersetWithID]

				// Update the new exercise with the new parent ID
				if err := tx.Model(&GymExercise{}).Where("id = ?", newExerciseID).Update("superset_with_id", newParentID).Error; err != nil {
					return err
				}
			}
		}

		return nil // Commit the transaction
	})

	return draftID, err
}

// FinalizeDraft promotes a draft to 'active'.
// If it's an edit of an existing workout, it updates the original and deletes the draft.
// It returns the ID of the final, active workout.
func (r *ActivityRepo) FinalizeDraft(draftID uint, notes string) (uint, error) {
	var draftActivity Activity
	// Preload the exercises from the draft so we can move them
	if err := r.DB.Preload("GymExercises").First(&draftActivity, draftID).Error; err != nil {
		return 0, err
	}

	// This is an edit of a previously existing workout
	if draftActivity.OriginalActivityID != nil {
		originalID := *draftActivity.OriginalActivityID
		err := r.DB.Transaction(func(tx *gorm.DB) error {
			// 1. Delete all old exercises and sets from the ORIGINAL workout
			if err := tx.Where("activity_id = ?", originalID).Delete(&GymExercise{}).Error; err != nil {
				return err
			}

			// 2. "Move" the draft's exercises over to the original workout
			if err := tx.Model(&GymExercise{}).Where("activity_id = ?", draftID).Update("activity_id", originalID).Error; err != nil {
				return err
			}

			// 3. Update the original workout's name and notes from the draft
			if err := tx.Model(&Activity{}).Where("id = ?", originalID).Updates(map[string]interface{}{
				"name":  draftActivity.Name,
				"notes": notes,
			}).Error; err != nil {
				return err
			}

			// 4. Delete the now-empty draft activity
			if err := tx.Delete(&Activity{}, draftID).Error; err != nil {
				return err
			}
			return nil
		})
		return originalID, err
	}

	// This is a new workout being finished for the first time
	err := r.DB.Model(&draftActivity).Updates(map[string]interface{}{
		"status": StatusActive,
		"notes":  notes,
	}).Error

	return draftActivity.ID, err
}

// DeleteActivityAndChildren deletes an Activity and all its descendant exercises and sets.
func (r *ActivityRepo) DeleteActivityAndChildren(activityID uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		// Find all exercise IDs for this activity
		var exerciseIDs []uint
		if err := tx.Model(&GymExercise{}).Where("activity_id = ?", activityID).Pluck("id", &exerciseIDs).Error; err != nil {
			return err
		}

		// If there are exercises, delete their sets
		if len(exerciseIDs) > 0 {
			if err := tx.Where("gym_exercise_id IN ?", exerciseIDs).Delete(&GymSet{}).Error; err != nil {
				return err
			}
		}

		// Delete the exercises
		if err := tx.Where("activity_id = ?", activityID).Delete(&GymExercise{}).Error; err != nil {
			return err
		}

		// Finally, delete the activity itself
		return tx.Delete(&Activity{}, activityID).Error
	})
}
