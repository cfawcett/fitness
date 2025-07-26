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
