#!/bin/bash

echo "Applying all database migrations"

CHAR_DB_HOST=${CHAR_DB_HOST:-localhost}

for f in ./*.sql; do
  echo "Running $f"
  mysql -h "$CHAR_DB_HOST" -uroot -ppassword acore_characters < "$f"
done