-- name: GetHashedPasswordByUser :one
SELECT hashed_password FROM users 
  WHERE ($1 = email);
