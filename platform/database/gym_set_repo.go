package database

import "gorm.io/gorm"

type GymSetRepo struct {
	DB *gorm.DB
}

// NewGymSetRepo creates a new GymSetRepo
func NewGymSetRepo(db *gorm.DB) *GymSetRepo {
	return &GymSetRepo{DB: db}
}

// CreateGymSet adds a new gym set to the database
func (r *GymSetRepo) CreateGymSet(gymset *GymSet) error {
	result := r.DB.Create(gymset)
	return result.Error
}

// GetSetsByActivityID returns a list of gym sets in a given activity
func (r *GymSetRepo) GetSetsByActivityID(activityID uint) ([]*GymSet, error) {
	var gymsets []*GymSet
	result := r.DB.Where("activity_id = ?", activityID).Preload("ExerciseDefinition").Order("id asc, set_number asc").Find(&gymsets)
	if result.Error != nil {
		return nil, result.Error
	}
	return gymsets, nil
}

// AddSetToActivity adds a new set to an activity
func (r *GymSetRepo) AddGymSetToExercise(set *GymSet) error {
	result := r.DB.Create(set)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

// CountByActivityID counts how many sets exist for a specific activity.
func (r *GymSetRepo) CountByActivityID(activityID uint64) (int64, error) {
	var count int64
	err := r.DB.Model(&GymSet{}).Where("activity_id = ?", activityID).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *GymSetRepo) GetPopulatedExercises(activityID uint) ([]PopulatedExercise[GymSet], error) {
	var allSets []GymSet
	err := r.DB.Model(&GymSet{}).
		Preload("ExerciseDefinition").
		Where("activity_id = ?", activityID).
		Order("exercise_definition_id, set_number ASC").
		Find(&allSets).Error

	if err != nil || len(allSets) == 0 {
		return nil, err
	}

	// Process the flat list into the grouped structure.
	var result []PopulatedExercise[GymSet]
	// Start with the first exercise
	currentExerciseID := allSets[0].ExerciseDefinitionID
	currentSets := []GymSet{}

	for _, set := range allSets {
		if set.ExerciseDefinitionID != currentExerciseID {
			// New exercise found, save the previous one
			result = append(result, PopulatedExercise[GymSet]{
				ExerciseID:   currentExerciseID,
				ExerciseName: currentSets[0].ExerciseDefinition.Name,
				Sets:         currentSets,
			})
			// And start a new group
			currentSets = []GymSet{}
			currentExerciseID = set.ExerciseDefinitionID
		}
		currentSets = append(currentSets, set)
	}
	// Append the very last group
	result = append(result, PopulatedExercise[GymSet]{
		ExerciseID:   currentExerciseID,
		ExerciseName: currentSets[0].ExerciseDefinition.Name,
		Sets:         currentSets,
	})

	return result, nil
}

// ReplaceActivitySets deletes all sets for an activity and creates new ones in a single transaction.
func (r *GymSetRepo) ReplaceActivitySets(activityID uint, newSets []*GymSet) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("activity_id = ?", activityID).Delete(&GymSet{}).Error; err != nil {
		}

		if len(newSets) > 0 {
			if err := tx.Create(&newSets).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetExerciseHistoryForUser fetches all sets for a specific exercise and user.
func (r *GymSetRepo) GetExerciseHistoryForUser(userID, exerciseID uint) ([]*GymSet, error) {
	var sets []*GymSet
	err := r.DB.Joins("JOIN activities ON activities.id = gym_sets.activity_id").
		Where("activities.user_id = ? AND gym_sets.exercise_definition_id = ?", userID, exerciseID).
		Order("activities.activity_time DESC, gym_sets.set_number ASC").
		Preload("Activity").
		Find(&sets).Error
	return sets, err
}
