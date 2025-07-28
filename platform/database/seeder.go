package database

//
//import "log"
//
//func SeedExercises(repo *ExerciseRepo) {
//	var count int64
//	repo.DB.Model(&ExerciseDefinition{}).Count(&count)
//
//	if count != 0 {
//		log.Println("No exercises to seed")
//		return
//	}
//
//	log.Println("Seeding exercises")
//
//	//exercises := []ExerciseDefinition{
//	//	{Name: "Barbell Bench Press", TargetMuscleGroup: "Chest"},
//	//	{Name: "Barbell Back Squat", TargetMuscleGroup: "Legs"},
//	//	{Name: "Barbell Overhead Press", TargetMuscleGroup: "Shoulders"},
//	//}
//	for _, exercise := range exercises {
//		if err := repo.CreateExercise(&exercise); err != nil {
//			log.Printf("Error seeding exercise %v: %v", exercise.Name, err)
//		}
//	}
//}
