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
  JSON="{\"transaction_id\": \"${ID}\", \"data\": [{\"wallet\": \"UQCY96AMzHb1JhdpZLTVUBk4bPwDlrSrp3onHIBr5k1r4625\", \"amount\": 0.0003, \"comment\": \"Payment for #${ID}\"},{\"wallet\": \"UQBdXuWUU-7WQp8r8t4OqL6cQ08E3hOno4xAKW67jK6sTsVH\", \"amount\": 0.0002, \"comment\": \"#${ID}\"}, {\"wallet\": \"UQCBMyUyNt2Q77tMuMWSmthDk8lE0UvPCWjJs_lPHz6E909s\", \"amount\": 0.0001, \"comment\": \"#${ID}\"}]}"

  echo "Posting: $JSON"
  
  curl -s -X POST "$API_URL" \
    -H "Content-Type: application/json" \
    -d "$JSON"
    
  echo  # Newline for readability
done

