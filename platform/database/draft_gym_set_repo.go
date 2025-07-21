package database

import "gorm.io/gorm"

type DraftGymSetRepo struct {
	DB *gorm.DB
}

// NewDraftGymSetRepo creates a new DraftGymSetRepo
func NewDraftGymSetRepo(db *gorm.DB) *DraftGymSetRepo {
	return &DraftGymSetRepo{DB: db}
}

// GetDraftSetsByActivityID returns a list of draft gym sets for a given activity.
func (r *DraftGymSetRepo) GetDraftSetsByActivityID(activityID uint) ([]*DraftGymSet, error) {
	var gymsets []*DraftGymSet
	result := r.DB.Where("activity_id = ?", activityID).Preload("ExerciseDefinition").Order("id asc").Find(&gymsets)
	if result.Error != nil {
		return nil, result.Error
	}
	return gymsets, nil
}

// CreateDraftSet adds a new draft set.
func (r *DraftGymSetRepo) CreateDraftSet(set *DraftGymSet) error {
	result := r.DB.Create(set)
	return result.Error
}

// GetPopulatedDraftExercises groups a flat list of draft sets into exercises.
// FIX: The function signature now correctly returns the generic type.
func (r *DraftGymSetRepo) GetPopulatedDraftExercises(activityID uint) ([]PopulatedExercise[DraftGymSet], error) {
	var allSets []DraftGymSet
	err := r.DB.Model(&DraftGymSet{}).
		Preload("ExerciseDefinition").
		Where("activity_id = ?", activityID).
		Order("id ASC").
		Find(&allSets).Error

	if err != nil {
		return nil, err
	}

	// FIX: The return variable is now correctly typed.
	var result []PopulatedExercise[DraftGymSet]
	if len(allSets) == 0 {
		return result, nil
	}

	// Grouping logic
	var currentSets []DraftGymSet
	currentExerciseID := allSets[0].ExerciseDefinitionID

	for _, set := range allSets {
		// This condition checks if we've moved to a new exercise.
		if set.ExerciseDefinitionID != currentExerciseID {
			// Save the previous group of sets before starting a new one.
			if len(currentSets) > 0 {
				// FIX: Correctly use the generic type when creating the struct.
				result = append(result, PopulatedExercise[DraftGymSet]{
					ExerciseID:   currentExerciseID,
					ExerciseName: currentSets[0].ExerciseDefinition.Name,
					Sets:         currentSets,
				})
			}
			// FIX: The reset logic now ONLY runs when the exercise changes.
			// It was previously outside this 'if' block, causing it to run on every loop.
			currentSets = []DraftGymSet{}
			currentExerciseID = set.ExerciseDefinitionID
		}
		// Add the current set to the current group.
		currentSets = append(currentSets, set)
	}

	// Append the very last group after the loop finishes.
	if len(currentSets) > 0 {
		// FIX: Correctly use the generic type for the final append.
		result = append(result, PopulatedExercise[DraftGymSet]{
			ExerciseID:   currentExerciseID,
			ExerciseName: currentSets[0].ExerciseDefinition.Name,
			Sets:         currentSets,
		})
	}

	return result, nil
}

// ReplaceActivityDraftSets deletes all draft sets for an activity and creates new ones.
func (r *DraftGymSetRepo) ReplaceActivityDraftSets(activityID uint, newSets []*DraftGymSet) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("activity_id = ?", activityID).Delete(&DraftGymSet{}).Error; err != nil {
			return err
		}

		if len(newSets) > 0 {
			if err := tx.Create(&newSets).Error; err != nil {
				return err
			}
		}

		return nil
	})
}
