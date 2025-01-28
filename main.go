package main

import (
	"log"
	"net/http"

    "project_sem/db"
	"project_sem/handlers"

	"github.com/gorilla/mux"
)

func main() {

    db.InitDB()

	r := mux.NewRouter()


	// Регистрация эндпоинтов
	r.HandleFunc("/api/v0/prices", handlers.UploadPrices).Methods("POST")
	r.HandleFunc("/api/v0/prices", handlers.GetPrices).Methods("GET")

	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
