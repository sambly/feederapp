# Этап сборки
FROM golang:1.22.5-alpine AS builder

# Установка необходимого для сборки
RUN apk add --no-cache git make 

# Установка рабочей директории
WORKDIR /app

# Копирование и установка зависимостей
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Выполнение команды make all
RUN make all

# Очистка зависимостей и сборка приложения
RUN go mod tidy && \
    go build -o myFeederApp ./cmd/grpc

# Минимальный финальный образ
FROM alpine:3.18

# Установка зависимостей для выполнения приложения
RUN apk add --no-cache ca-certificates


# Установка зависимостей для выполнения приложения
RUN apk add --no-cache ca-certificates tzdata \
    && ln -sf /usr/share/zoneinfo/Europe/Moscow /etc/localtime \
    && echo "Europe/Moscow" > /etc/timezone


# Устанавливаем рабочую директорию в контейнере
WORKDIR /app

COPY --from=builder /app/myFeederApp .

# Создание точки монтирования для логов
VOLUME /app/log

# Команда для запуска вашего приложения
CMD ["./myFeederApp"]
