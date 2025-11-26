#!/bin/bash

echo "Starting mixed traffic generator..."
echo "Press Ctrl+C to stop"

#Base URL for Cloud Run deployment
BASE_URL="https://stickermule-app-386055911814.us-central1.run.app"

while true; do
  #Random number between 1-100 to determine behavior
  RAND=$((RANDOM % 100))
  
  if [ $RAND -lt 60 ]; then
    #60% chance: Successful requests
    curl -s $BASE_URL/health > /dev/null
    curl -s "$BASE_URL/store?id=1" > /dev/null
  elif [ $RAND -lt 80 ]; then
    #20% chance: 404 errors (invalid store ID)
    curl -s "$BASE_URL/store?id=9999" > /dev/null
  elif [ $RAND -lt 90 ]; then
    #10% chance: More 404s (burst of errors)
    for i in {1..3}; do
      curl -s "$BASE_URL/store?id=9999" > /dev/null
    done
  else
    #10% chance: Mixed burst (success + errors)
    curl -s $BASE_URL/health > /dev/null
    curl -s "$BASE_URL/store?id=1" > /dev/null
    curl -s "$BASE_URL/store?id=9999" > /dev/null
  fi
  
  #Random sleep between 0.1 and 0.5 seconds
  sleep 0.$((RANDOM % 5))
done