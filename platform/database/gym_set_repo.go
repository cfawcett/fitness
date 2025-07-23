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
