#!/bin/bash

echo "Starting mixed traffic generator..."
echo "Press Ctrl+C to stop"

while true; do
  #Random number between 1-100 to determine behavior
  RAND=$((RANDOM % 100))
  
  if [ $RAND -lt 60 ]; then
    #60% chance: Successful requests
    curl -s http://localhost:8080/health > /dev/null
    curl -s "http://localhost:8080/store?id=1" > /dev/null
  elif [ $RAND -lt 80 ]; then
    #20% chance: 404 errors (invalid store ID)
    curl -s "http://localhost:8080/store?id=9999" > /dev/null
  elif [ $RAND -lt 90 ]; then
    #10% chance: More 404s (burst of errors)
    for i in {1..3}; do
      curl -s "http://localhost:8080/store?id=9999" > /dev/null
    done
  else
    #10% chance: Mixed burst (success + errors)
    curl -s http://localhost:8080/health > /dev/null
    curl -s "http://localhost:8080/store?id=1" > /dev/null
    curl -s "http://localhost:8080/store?id=9999" > /dev/null
  fi
  
  #Random sleep between 0.1 and 0.5 seconds
  sleep 0.$((RANDOM % 5))
done