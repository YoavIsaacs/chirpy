-- name: GetAllChirps :many
SELECT * FROM chirps
  ORDER BY updated_at;
