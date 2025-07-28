// File: /scripts/seed/main.go

package main

import (
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"io/ioutil"
	"log"
	"strings"

	// IMPORTANT: Change this import path to match your project's database package
	"fitness/platform/database"
)

// A temporary struct to perfectly match the JSON file's structure
type ExerciseJSON struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	PrimaryMuscles   []string `json:"primaryMuscles"`
	Equipment        string   `json:"equipment"`
	GifURL           string   `json:"gifUrl"`
	BodyPart         string   `json:"bodyPart"`
	SecondaryMuscles []string `json:"secondaryMuscles"`
}

func main() {

	if err := godotenv.Load(); err != nil {
		log.Fatalf("Failed to load the env vars: %v", err)
	}
	log.Println("Starting database seeding for exercises...")

	db, err := database.NewDatabaseConnection()
	if err != nil {
		log.Fatalf("FATAL: Could not connect to the database: %v", err)
	}

	// Automatically migrate the schema to ensure new columns exist
	log.Println("Migrating ExerciseDefinition schema...")
	db.AutoMigrate(&database.ExerciseDefinition{})

	// Read the source JSON file
	byteValue, err := ioutil.ReadFile("./exercises.json") // Assumes exercises.json is in the same folder
	if err != nil {
		log.Fatalf("FATAL: Could not read exercises.json file: %v", err)
	}

	var exercises []ExerciseJSON
	json.Unmarshal(byteValue, &exercises)

	log.Printf("Found %d exercises in the JSON file. Seeding...", len(exercises))

	var seededCount int
	for _, ex := range exercises {
		// Construct the local path to the GIF, assuming Gin serves /public at /static
		// and the GIF filename matches the ID from the JSON.
		imageStartPath := fmt.Sprintf("/static/exercises/%s/0.jpg", ex.ID)
		imageEndPath := fmt.Sprintf("/static/exercises/%s/1.jpg", ex.ID)

		caser := cases.Title(language.English)
		for i, muscle := range ex.SecondaryMuscles {
			ex.SecondaryMuscles[i] = caser.String(muscle)
		}

		if len(ex.PrimaryMuscles) > 0 {
			ex.PrimaryMuscles[0] = caser.String(ex.PrimaryMuscles[0])
		}

		// Create a new record using your actual GORM model
		exerciseDef := database.ExerciseDefinition{
			Name:               strings.Title(ex.Name), // Capitalize the name nicely
			PrimaryMuscleGroup: ex.PrimaryMuscles[0],
			BodyPart:           strings.Title(ex.BodyPart),
			Equipment:          strings.Title(ex.Equipment),
			ImageUrlStart:      imageStartPath,
			ImageUrlEnd:        imageEndPath,
			SecondaryMuscles:   ex.SecondaryMuscles,
		}

		// Use FirstOrCreate to avoid duplicating exercises if the script is run again
		result := db.Where(database.ExerciseDefinition{Name: exerciseDef.Name}).FirstOrCreate(&exerciseDef)

		if result.Error != nil {
			log.Printf("WARN: Could not insert exercise '%s': %v\n", ex.Name, result.Error)
		} else if result.RowsAffected > 0 {
			seededCount++
		}
	}

	log.Printf("Database seeding completed. Added %d new exercises.", seededCount)
}
