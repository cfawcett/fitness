package database

import (
	"gorm.io/gorm"
)

type GymExerciseRepo struct {
	DB *gorm.DB
}

// NewGymExerciseRepo creates a new GymExerciseRepo
func NewGymExerciseRepo(db *gorm.DB) *GymExerciseRepo {
	return &GymExerciseRepo{DB: db}
}

// CreateGymExercise adds a new gym exercise to the database
func (r *GymExerciseRepo) CreateGymExercise(gymExercise *GymExercise) error {
	result := r.DB.Create(gymExercise)
	return result.Error
}

// GetGymExerciseByID returns an exercise based on its ID
func (r *GymExerciseRepo) GetGymExerciseByID(gymExerciseID uint) (*GymExercise, error) {
	var result GymExercise
	// Use Preload to also fetch the associated sets
	err := r.DB.Preload("Sets").First(&result, gymExerciseID).Error
	return &result, err
}

// GetExercisesBySupersetID returns all exercises belonging to a specific superset, ordered correctly.
func (r *GymExerciseRepo) GetExercisesBySupersetID(supersetID string) ([]GymExercise, error) {
	var results []GymExercise
	err := r.DB.Preload("Sets").
		Where("superset_id = ?", supersetID).
		Order("superset_order asc").
		Find(&results).Error
	return results, err
}

// UpdateGymExercise updates an existing GymExercise's details.
func (r *GymExerciseRepo) UpdateGymExercise(gymExercise *GymExercise) error {
	return r.DB.Save(gymExercise).Error
}

// GetExercisesByActivityId returns a list of gym exercises in a given activity
func (r *GymExerciseRepo) GetExercisesByActivityId(activityID uint) ([]*GymExercise, error) {
	var gymExercises []*GymExercise
	// Order by the main sort number for the workout flow
	result := r.DB.Where("activity_id = ?", activityID).Order("sort_number asc").Find(&gymExercises)
	return gymExercises, result.Error
}

// CountByActivityID counts how many exercises exist for a specific activity.
func (r *GymExerciseRepo) CountByActivityID(activityID uint) (int64, error) {
	var count int64
	err := r.DB.Model(&GymExercise{}).Where("activity_id = ?", activityID).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// DeleteExercise deletes a GymExercise and all of its child sets in a transaction.
func (r *GymExerciseRepo) DeleteExercise(id uint) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		// First, delete all associated sets.
		if err := tx.Where("gym_exercise_id = ?", id).Delete(&GymSet{}).Error; err != nil {
			return err
		}

		// Then, delete the exercise itself.
		if err := tx.Delete(&GymExercise{}, id).Error; err != nil {
			return err
		}

		return nil
	})
}
