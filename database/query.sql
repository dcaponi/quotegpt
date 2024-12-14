-- name: GetQuote :one
SELECT (id, author, content) FROM quotes
WHERE id = $1 LIMIT 1;

-- name: ListQuotes :many
SELECT (id, author, content) FROM quotes
ORDER BY author;

-- name: SearchQuotesByAuthor :many
SELECT (id, author, content) FROM quotes
WHERE author = $1;

-- name: SearchQuotes :many
SELECT (id, author, content) FROM quotes
ORDER BY embedding <=> $1
LIMIT 5;

-- name: CreateQuote :one
INSERT INTO quotes (
  content, author, embedding
) VALUES (
  $1, $2, $3
)
RETURNING (id, author, content);

-- name: UpdateQuote :exec
UPDATE quotes
  set content = $2,
  author = $3,
  embedding = $4
WHERE id = $1
RETURNING (id, author, content);

-- name: DeleteQuote :exec
DELETE FROM quotes
WHERE id = $1;