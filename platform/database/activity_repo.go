package database

import "gorm.io/gorm"

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

// GetActivityById returns the activity based on its database id
func (r *ActivityRepo) GetActivityByID(id uint) (*Activity, error) {
	var activity Activity
	result := r.DB.First(&activity, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &activity, nil
}

// UpdateActivityStatus updates the status of a specific activity.
func (r *ActivityRepo) UpdateActivityStatus(activityID uint, status ExerciseStatus) error {
	err := r.DB.Model(&Activity{}).Where("id = ?", activityID).Update("status", status).Error
	return err
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
