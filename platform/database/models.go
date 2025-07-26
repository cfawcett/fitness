package database

import (
	"fmt"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Migrate creates or updates database tables to match the GORM models.
func Migrate(db *gorm.DB) error {
	createEnumSQL := `
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'exercise_status') THEN
        CREATE TYPE exercise_status AS ENUM ('draft', 'active', 'archived');
    END IF;
END$$;
`
	if err := db.Exec(createEnumSQL).Error; err != nil {
		return fmt.Errorf("failed to create enum type: %w", err)
	}

	err := db.AutoMigrate(
		&User{},
		&Activity{},
		&ExerciseDefinition{},
		&GymSet{},
		&GymExercise{},
	)
	return err
}

type User struct {
	gorm.Model

	Auth0Sub          string `gorm:"uniqueIndex"`
	Username          string `gorm:"uniqueIndex;size:50"`
	Email             string `gorm:"uniqueIndex;size:255"`
	FirstName         string `gorm:"size:100"`
	LastName          string `gorm:"size:100"`
	Bio               string
	Location          string `gorm:"size:255"`
	ProfilePictureUrl string
	Dob               time.Time
	Gender            string `gorm:"size:50"`
	PrimarySport      string `gorm:"size:100"`
	HeightCM          int
	CurrentWeightKG   float64
	UnitSystem        string `gorm:"size:10"`
	IsPT              bool   `gorm:"default:false"`
}

type Activity struct {
	gorm.Model
	UserID             uint
	User               User           `gorm:"foreignKey:UserID"`
	Type               string         `gorm:"size:50;not null"`
	ActivityTime       time.Time      `gorm:"not null"`
	Name               string         `gorm:"size:255"`
	Details            pq.StringArray `gorm:"type:jsonb"`
	Status             ExerciseStatus `gorm:"type:exercise_status;default:'draft';not null"`
	OriginalActivityID *uint          `gorm:"index"`

	GymExercises []GymExercise `gorm:"foreignKey:ActivityID"`
}

type ExerciseDefinition struct {
	gorm.Model
	Name              string
	Description       string
	VideoUrl          string
	ImageUrl          string
	TargetMuscleGroup string `gorm:"size:100"`
}

type GymExercise struct {
	gorm.Model
	ActivityID           uint
	ExerciseDefinitionID uint
	SupersetPartnerID    *uint

	Activity           Activity           `gorm:"foreignKey:ActivityID"`
	ExerciseDefinition ExerciseDefinition `gorm:"foreignKey:ExerciseDefinitionID"`
	SortNumber         int                `gorm:"not null"`
	SupersetPartner    *GymExercise       `gorm:"foreignKey:SupersetPartnerID"`
	Sets               []GymSet           `gorm:"foreignKey:GymExerciseID"`
}

type GymSet struct {
	gorm.Model

	GymExerciseID uint
	GymExercise   *GymExercise `gorm:"foreignKey:GymExerciseID"`
	SetNumber     int          `gorm:"not null" json:"set_number"`
	Reps          int          `gorm:"not null" json:"reps"`
	WeightKG      float64      `gorm:"not null" json:"weight"`
	SetType       string       `gorm:"size:50" json:"set_type"`
}
