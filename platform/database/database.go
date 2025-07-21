package database

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
)

// NewDatabaseConnection handles creating the connection to the database.
func NewDatabaseConnection() (*gorm.DB, error) {
	dbHost := os.Getenv("DATABASE_HOST")
	dbUser := os.Getenv("DATABASE_USER")
	dbPassword := os.Getenv("DATABASE_PASSWORD")
	dbName := os.Getenv("DATABASE_NAME")
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=5432 sslmode=require", dbHost, dbUser, dbPassword, dbName)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return db, nil
}
