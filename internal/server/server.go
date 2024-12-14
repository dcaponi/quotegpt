package server

import (
	"fmt"
	"net/http"
	"quoteGPT/internal/database"
	"time"
)

type Server struct {
	Port int

	DB *database.Queries
}

func NewServer(s *Server) *http.Server {
	// Declare Server config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.Port),
		Handler:      s.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}
