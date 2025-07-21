package database

// ExerciseStatus is our custom "enum" type for the status of an exercise.
type ExerciseStatus string

// Define the possible values for the ExerciseStatus enum.
const (
	StatusDraft    ExerciseStatus = "draft"
	StatusActive   ExerciseStatus = "active"
	StatusArchived ExerciseStatus = "archived"
)
