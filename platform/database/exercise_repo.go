package database

import "gorm.io/gorm"

type ExerciseRepo struct {
	DB *gorm.DB
}

// NewExerciseRepo creates a new ExerciseRepo
func NewExerciseRepo(db *gorm.DB) *ExerciseRepo {
	return &ExerciseRepo{DB: db}
}

// CreateExercise adds a new exercise to the database
func (r *ExerciseRepo) CreateExercise(exercise *ExerciseDefinition) error {
	result := r.DB.Create(exercise)
	return result.Error
}

// GetExerciseByID returns an exercise by its database id number
func (r *ExerciseRepo) GetExerciseByID(exerciseID uint) (*ExerciseDefinition, error) {
	var exercise *ExerciseDefinition

	result := r.DB.First(&exercise, exerciseID)

	if result.Error != nil {
		return nil, result.Error
	}
	return exercise, nil
}

// GetExerciseList returns all exercises in the database
func (r *ExerciseRepo) GetExerciseList() ([]*ExerciseDefinition, error) {
	var exercises []*ExerciseDefinition

	result := r.DB.Find(&exercises)

	if result.Error != nil {
		return nil, result.Error
	}
	return exercises, nil
}
