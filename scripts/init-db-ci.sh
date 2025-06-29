#!/usr/bin/env bash
set -e

# Wait until PostgreSQL is ready
for i in {1..20}; do
  if pg_isready -h localhost -p 5432 -U root; then break; fi
  echo "Waiting for PostgreSQL..." && sleep 2
done

pg_isready -h localhost -p 5432 -U root || {
  echo "PostgreSQL did not become ready in time"
  exit 1
}

# Create databases per service
for dir in services/*/; do
  db=$(basename "$dir")
  echo "Creating database: $db"
  PGPASSWORD=password psql -h localhost -U root -d postgres -c "CREATE DATABASE \"$db\";"
done
