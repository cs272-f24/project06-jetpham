#!/bin/bash

# Check if the server on port 5000 is running
if ! nc -z localhost 5000; then
  echo "Starting server on port 5000..."
  nohup python app.py &> app.log &
  sleep 5 # Give the server some time to start
else
  echo "Server on port 5000 is already running."
fi

# Check if the Docker container is up
if ! docker compose ps | grep 'Up'; then
  echo "Starting Docker container..."
  docker compose up -d
else
  echo "Docker container is already running."
fi

# Run the Go program
echo "Running Go program..."
go run .