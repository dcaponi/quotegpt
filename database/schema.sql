CREATE EXTENSION IF NOT EXISTS "vector";

CREATE TABLE IF NOT EXISTS quotes (
  id   BIGSERIAL PRIMARY KEY,
  content   text      NOT NULL,
  author    text      NOT NULL,
  embedding vector(1536)
);

CREATE INDEX idx_author 
ON quotes (author);

CREATE INDEX idx_embedding 
ON quotes 
USING ivfflat (embedding vector_cosine_ops) 
WITH (lists = 100);