package database

//
import "gorm.io/gorm"

type GymExerciseRepo struct {
	DB *gorm.DB
}

// NewGymExerciseRepo creates a new GymSetRepo
func NewGymExerciseRepo(db *gorm.DB) *GymExerciseRepo {
	return &GymExerciseRepo{DB: db}
}

// CreateGymExercise adds a new gym set to the database
func (r *GymExerciseRepo) CreateGymExercise(gymExercise *GymExercise) error {
	result := r.DB.Create(gymExercise)
	return result.Error
}

// GetExerciseById returns an exercise based on its ID
func (r *GymExerciseRepo) GetExerciseByID(gymExerciseID uint64) (*GymExercise, error) {
	var result GymExercise
	err := r.DB.First(&result, gymExerciseID).Error
	return &result, err
}

// In your database/gym_exercise_repo.go
func (r *GymExerciseRepo) IncrementSortFrom(activityID uint, sortNumber int) error {
	return r.DB.Model(&GymExercise{}).
		Where("activity_id = ? AND sort_number > ?", activityID, sortNumber).
		Update("sort_number", gorm.Expr("sort_number + 1")).Error
}

// UpdateSupersetInfo updates an existing GymExercise with its new superset details.
func (r *GymExerciseRepo) UpdateSupersetInfo(gymExerciseID uint, supersetID *string, order int) error {
	return r.DB.Model(&GymExercise{}).
		Where("id = ?", gymExerciseID).
		Updates(map[string]interface{}{"superset_id": supersetID, "superset_order": order}).Error
}

// GetNextSupersetOrder finds the highest order number in a superset and returns the next one.
func (r *GymExerciseRepo) GetNextSupersetOrder(activityID uint, supersetID string) (int, error) {
	var maxOrder int
	// COALESCE is a safe way to handle the case where no exercises exist yet, defaulting to -1.
	err := r.DB.Model(&GymExercise{}).
		Where("activity_id = ? AND superset_id = ?", activityID, supersetID).
		Select("COALESCE(MAX(superset_order), -1)").
		Row().
		Scan(&maxOrder)

	if err != nil {
		return 0, err
	}
	return maxOrder + 1, nil
}

// GetSupersetGroup fetches all exercises belonging to a specific superset within a workout.
func (r *GymExerciseRepo) GetSupersetGroup(activityID uint, supersetID string) ([]*GymExercise, error) {
	var group []*GymExercise
	err := r.DB.
		Where("activity_id = ? AND superset_id = ?", activityID, supersetID).
		Order("superset_order ASC").
		Preload("ExerciseDefinition"). // Eager load the name of the exercise
		Preload("Sets").               // Eager load the sets for each exercise
		Find(&group).Error
	return group, err
}

// GetExercisesByActivityId returns a list of gym exercises in a given activity
func (r *GymExerciseRepo) GetExercisesByActivityId(activityID uint) ([]*GymExercise, error) {
	var gymExercise []*GymExercise
	result := r.DB.Where("activity_id = ?", activityID).Order("set_number asc").Find(&gymExercise)
	return gymExercise, result.Error
}

// UpdateExercise updates a gym exercise in the database
func (r *GymExerciseRepo) UpdateExercise(gymExercise *GymExercise) error {
	result := r.DB.Model(&GymExercise{}).Updates(gymExercise)
	return result.Error
}

// CountByActivityID counts how many exercises exist for a specific activity.
func (r *GymExerciseRepo) CountByActivityID(activityID uint64) (int64, error) {
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
		// 1. Check if the exercise we are deleting has a child.
		var childExercise GymExercise
		err := tx.Where("superset_with_id = ?", id).First(&childExercise).Error

		// If a child was found...
		if err == nil {
			// 2. Promote the child by removing its link to the parent.
			if err := tx.Model(&childExercise).Update("superset_with_id", nil).Error; err != nil {
				return err
			}
		} else if err != gorm.ErrRecordNotFound {
			// If there was an actual error (not just "not found"), return it.
			return err
		}

		// 3. Now, delete all sets belonging to the exercise being deleted.
		if err := tx.Where("gym_exercise_id = ?", id).Delete(&GymSet{}).Error; err != nil {
			return err
		}

		// 4. Finally, delete the exercise itself.
		return tx.Delete(&GymExercise{}, id).Error
	})
}
