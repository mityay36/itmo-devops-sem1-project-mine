package db

import (
    "log"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() {
    dbHost := getEnv("POSTGRES_HOST", "localhost")
	dbPort := getEnv("POSTGRES_PORT", "5432")
	dbUser := getEnv("POSTGRES_USER", "validator")
	dbPassword := getEnv("POSTGRES_PASSWORD", "val1dat0r")
	dbName := getEnv("POSTGRES_DB", "project-sem-1")


	var err error
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to the database: %v", err))
	}

	err = DB.Ping()
	if err != nil {
		log.Fatalf("База данных недоступна: %v", err)
	}

	log.Println("Соединение с базой данных успешно установлено!")
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
