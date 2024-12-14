package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"quoteGPT/internal/embedding"
	"quoteGPT/internal/pages"
	"strconv"
	"time"

	"github.com/pgvector/pgvector-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *Server) RegisterRoutes() http.Handler {

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/", s.HelloWorldHandler)
	mux.HandleFunc("/quotes", hitCounterMiddleware(latencyMiddleware(enableCors(s.ListQuotesHandler))))
	mux.HandleFunc("/quote/{id}", hitCounterMiddleware(latencyMiddleware(enableCors(s.GetQuoteHandler))))
	mux.HandleFunc("/page/", hitCounterMiddleware(latencyMiddleware(enableCors(s.PageHandler))))

	return mux
}

func (s *Server) PageHandler(w http.ResponseWriter, r *http.Request) {
	pages.SearchPage().Render(r.Context(), w)
}

func (s *Server) HelloWorldHandler(w http.ResponseWriter, r *http.Request) {
	resp := make(map[string]string)
	resp["message"] = "Hola Mundo"

	jsonResp, err := json.Marshal(resp)
	if err != nil {
		log.Fatalf("error handling JSON marshal. Err: %v", err)
	}

	json.NewEncoder(w).Encode(jsonResp)
}

func (s *Server) ListQuotesHandler(w http.ResponseWriter, r *http.Request) {
	var quotes []interface{}
	var err error

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	author := r.URL.Query().Get("author")
	query := r.URL.Query().Get("query")
	if author != "" {
		quotes, err = s.quotesByAuthor(ctx, author)
	} else if query != "" {
		quotes, err = s.searchQuotes(ctx, query)
	} else {
		quotes, err = s.DB.ListQuotes(ctx)
	}
	if err != nil {
		message := fmt.Sprintf("%s", err)
		json.NewEncoder(w).Encode(map[string]string{"error": message})
		return
	}
	json.NewEncoder(w).Encode(quotes)
}

func (s *Server) GetQuoteHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
	defer cancel()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "id must be an integer"})
		return
	}
	quote, err := s.DB.GetQuote(ctx, id)
	if err != nil {
		message := fmt.Sprintf("%s", err)
		json.NewEncoder(w).Encode(map[string]string{"error": message})
		return
	}
	json.NewEncoder(w).Encode(quote)
}

func (s *Server) searchQuotes(ctx context.Context, query string) ([]interface{}, error) {
	queryEmbed, err := embedding.OpenAI(ctx, query)
	if err != nil {
		return []interface{}{}, err
	}
	return s.DB.SearchQuotes(ctx, pgvector.NewVector(queryEmbed))
}

func (s *Server) quotesByAuthor(ctx context.Context, author string) ([]interface{}, error) {
	return s.DB.SearchQuotesByAuthor(ctx, author)
}
