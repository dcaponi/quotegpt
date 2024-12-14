package database

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"quoteGPT/internal/embedding"

	"github.com/pgvector/pgvector-go"
)

type SourceQuote struct {
	ID     int    `json:"id"`
	Quote  string `json:"quote"`
	Author string `json:"author"`
}

type Response struct {
	Quotes []SourceQuote `json:"quotes"`
	Total  int           `json:"total"`
	Skip   int           `json:"skip"`
	Limit  int           `json:"limit"`
}

func Seed() {
	const quoteSrcURL = "https://dummyjson.com/quotes?limit=1500"

	ctx := context.Background()
	dbConn, err := PGPoolConn(ctx)
	if err != nil {
		log.Fatalln("unable to connect to database", err)
	}
	defer dbConn.Close()

	repo := New(dbConn)

	resp, err := http.Get(quoteSrcURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching data: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading response: %v\n", err)
		os.Exit(1)
	}

	var quotesResponse Response
	if err := json.Unmarshal(body, &quotesResponse); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	for _, srcQuote := range quotesResponse.Quotes {
		emb, err := embedding.OpenAI(ctx, srcQuote.Quote)
		if err != nil {
			log.Println("unable to create embed", err)
		}
		q := CreateQuoteParams{
			Content:   srcQuote.Quote,
			Author:    srcQuote.Author,
			Embedding: pgvector.NewVector(emb),
		}
		repo.CreateQuote(ctx, q)
	}
}
