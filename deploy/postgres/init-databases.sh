#!/bin/sh
# Every service owns its own database. This runs once, when the data volume is created.
set -e

for db in auth booking notification; do
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -c "CREATE DATABASE $db"
done
