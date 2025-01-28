#!/bin/bash

echo "Запуск сервера..."
go run main.go &
SERVER_PID=$!

# Ждём несколько секунд, чтобы сервер успел запуститься
sleep 5

echo "Сервер запущен с PID $SERVER_PID"
