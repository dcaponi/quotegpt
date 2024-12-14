package database

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	dbName     = os.Getenv("POSTGRES_DB")
	dbpassword = os.Getenv("POSTGRES_PASSWORD")
	dbusername = os.Getenv("POSTGRES_USERNAME")
	dbport     = os.Getenv("POSTGRES_PORT")
	dbhost     = os.Getenv("POSTGRES_HOST")
)

func PGPoolConn(ctx context.Context) (*pgxpool.Pool, error) {

	isLocal := os.Getenv("APP_ENV") == "local"
	sslDisable := ""
	if isLocal {
		sslDisable = "?sslmode=disable"
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s%s", dbusername, dbpassword, dbhost, dbport, dbName, sslDisable)
	dbConn, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %e", err)
	}

	return dbConn, err
}
