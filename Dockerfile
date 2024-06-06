# Этап сборки
FROM golang:1.21-alpine3.18 AS builder

# Установка необходимого для сборки
RUN apk add --no-cache git

# Установка рабочей директории
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
ADD . .
RUN go mod tidy

# Сборка приложения
RUN go build -o myFeederApp .

# Минимальный финальный образ
FROM alpine:3.18

# Установка зависимостей для выполнения приложения
RUN apk add --no-cache ca-certificates

# Устанавливаем рабочую директорию в контейнере
WORKDIR /app

COPY --from=builder /app/myFeederApp .

# Создание точки монтирования для логов
VOLUME /app/log

# Команда для запуска вашего приложения
CMD ["./myFeederApp"]
