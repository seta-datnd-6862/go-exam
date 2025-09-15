#!/bin/sh
set -e

echo "Waiting for Postgres to be ready..."
until pg_isready -h ${DB_HOST:-postgres} -p ${DB_PORT:-5432} -U ${DB_USER:-blog}; do
  sleep 1
done

echo "Running migrations..."
psql "postgresql://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE:-disable}" -f /app/db/migrations.sql
echo "Migrations done."
