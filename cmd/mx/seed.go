package main

import (
	"log"
	"quoteGPT/internal/database"
)

func main() {
	log.Println("Seeding the database")
	database.Seed()
	log.Println("Finished seeding")
}
