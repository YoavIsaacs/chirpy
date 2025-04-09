-- name: CreateChirp :one
INSERT INTO chirps (
  id, body, user_id
) VALUES (gen_random_uuid(), $1, $2)
  RETURNING *;
