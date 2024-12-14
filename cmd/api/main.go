package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"quoteGPT/internal/database"
	"quoteGPT/internal/server"
	"strconv"
)

func main() {

	ctx := context.Background()
	port, _ := strconv.Atoi(os.Getenv("PORT"))

	dbConn, err := database.PGPoolConn(ctx)
	if err != nil {
		log.Fatalln("unable to connect to database!", err)
	}

	defer dbConn.Close()

	// database.Seed() // uncomment this to seed postgres data if using docker-compose
	s := &server.Server{
		Port: port,

		DB: database.New(dbConn),
	}

	server := server.NewServer(s)

	err = server.ListenAndServe()
	if err != nil {
		panic(fmt.Sprintf("cannot start server: %s", err))
	}
}
