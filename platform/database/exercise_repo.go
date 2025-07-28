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

// SearchExercises performs a filtered and sorted search for exercises.
// It prioritizes exercises favorited by the user.
func (r *ExerciseRepo) SearchExercises(userID uint, search, muscleGroup string) ([]ExerciseDefinition, error) {
	var exercises []ExerciseDefinition

	query := r.DB.
		// Use a LEFT JOIN to see which exercises are in the user's favorites
		Joins("LEFT JOIN favourite_exercises ON favourite_exercises.exercise_definition_id = exercise_definitions.id AND favourite_exercises.user_id = ?", userID)

	if search != "" {
		// Add a search filter if a search term is provided
		query = query.Where("exercise_definitions.name ILIKE ?", "%"+search+"%")
	}

	if muscleGroup != "" {
		// Add a muscle group filter if one is selected
		query = query.Where("primary_muscle_group = ?", muscleGroup)
	}

	// This is the key for sorting:
	// 1. Put favorited exercises first (where user_id is NOT NULL).
	// 2. Then, sort all exercises alphabetically by name.
	err := query.Order("CASE WHEN favourite_exercises.user_id IS NOT NULL THEN 0 ELSE 1 END, exercise_definitions.name ASC").
		Find(&exercises).Error

	return exercises, err
}

func (r *ExerciseRepo) GetUniqueMuscleGroups() ([]string, error) {
	var muscleGroups []string

	err := r.DB.Model(&ExerciseDefinition{}).
		Distinct("primary_muscle_group").
		Pluck("primary_muscle_group", &muscleGroups).Error

	return muscleGroups, err
}
