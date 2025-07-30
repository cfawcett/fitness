package workout

import (
	"fitness/platform/database"
	"fitness/platform/models"
	"sort"
)

// GroupExercisesIntoBlocks converts a flat list of exercises into grouped blocks.
func GroupExercisesIntoBlocks(exercises []database.GymExercise) []*models.RenderableExerciseBlock {
	sort.Slice(exercises, func(i, j int) bool {
		return exercises[i].SortNumber < exercises[j].SortNumber
	})

	blocksByID := make(map[string]*models.RenderableExerciseBlock)
	var orderedBlocks []*models.RenderableExerciseBlock
	processed := make(map[uint]bool) // Tracks exercises already added to a block

	for _, ex := range exercises {
		if processed[ex.ID] {
			continue
		}

		if ex.SupersetID != nil && *ex.SupersetID != "" {
			supersetID := *ex.SupersetID

			if _, ok := blocksByID[supersetID]; !ok {
				newBlock := &models.RenderableExerciseBlock{Exercises: []database.GymExercise{}}
				blocksByID[supersetID] = newBlock
				orderedBlocks = append(orderedBlocks, newBlock)
			}

			block := blocksByID[supersetID]
			for _, subEx := range exercises {
				if subEx.SupersetID != nil && *subEx.SupersetID == supersetID {
					block.Exercises = append(block.Exercises, subEx)
					processed[subEx.ID] = true
				}
			}
			sort.Slice(block.Exercises, func(i, j int) bool {
				return block.Exercises[i].SupersetOrder < block.Exercises[j].SupersetOrder
			})

		} else {
			// It's a standalone exercise, so it gets its own block.
			newBlock := &models.RenderableExerciseBlock{
				Exercises: []database.GymExercise{ex},
			}
			orderedBlocks = append(orderedBlocks, newBlock)
			processed[ex.ID] = true
		}
	}

	return orderedBlocks
}
