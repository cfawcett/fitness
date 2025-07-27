package database

import "gorm.io/gorm"

type UserRepo struct {
	DB *gorm.DB
}

// NewUserRepo creates a new UserRepo
func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{DB: db}
}

// CreateUser adds a new user to the database
func (r *UserRepo) CreateUser(user *User) error {
	result := r.DB.Create(user)
	return result.Error
}

// GetUserByAuthID finds a user by their unique Auth0 Sub ID
func (r *UserRepo) GetUserByAuthID(authID string) (*User, error) {
	var user User
	result := r.DB.Where("auth0_sub = ?", authID).First(&user)
	if result.Error != nil {
		return nil, result.Error // Probably gorm.ErrRecordNotFound
	}
	return &user, nil
}

func (r *UserRepo) GetUserById(id uint64) (*User, error) {
	var user User
	result := r.DB.Where("id = ?", id).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func (r *UserRepo) UpdateUser(user *User) error {
	result := r.DB.Updates(user)
	return result.Error
}
