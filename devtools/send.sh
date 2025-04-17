#!/bin/bash

# Check if the number of iterations was provided
if [ -z "$1" ]; then
  echo "Usage: $0 <number_of_iterations>"
  exit 1
fi

ITERATIONS=$1
API_URL="http://127.0.0.1:8090/ton/send"

generate_id() {
  tr -dc A-Za-z0-9 </dev/urandom | head -c 10
}

for ((i = 1; i <= ITERATIONS; i++)); do
  ID=$(generate_id)
  JSON="{\"pattern\": \"send-ton\", \"data\": {\"transaction_id\": \"${ID}\", \"data\": [{\"wallet\": \"0QBaxcn6UaXPNCWX0tasT5QpeBcQvITvt2FNEzXlERYa8qWe\", \"amount\": 10.0003, \"comment\": \"Payment for #${ID}\"},{\"wallet\": \"0QBaxcn6UaXPNCWX0tasT5QpeBcQvITvt2FNEzXlERYa8qWe\", \"amount\": 0.0002, \"comment\": \"#${ID}\"}, {\"wallet\": \"0QBaxcn6UaXPNCWX0tasT5QpeBcQvITvt2FNEzXlERYa8qWe\", \"amount\": 0.0001, \"comment\": \"#${ID}\"}]}}"

  echo "Posting: $JSON"
  
  curl -s -X POST "$API_URL" \
    -H "Content-Type: application/json" \
    -d "$JSON"
    
  echo  # Newline for readability
done

