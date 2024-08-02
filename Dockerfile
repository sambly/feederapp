# Этап сборки
FROM golang:1.22.5-alpine AS builder

# Установка необходимого для сборки
RUN apk add --no-cache git make 

# Установка рабочей директории
WORKDIR /app

# Передача аргументов сборки
ARG GITHUB_TOKEN
ARG ENVIRONMENT


# Установка сборочных аргументов как переменных окружения
ENV GITHUB_TOKEN=$GITHUB_TOKEN
ENV ENVIRONMENT=$ENVIRONMENT

# Выполнение команды make all
COPY Makefile ./
COPY go.mod go.sum ./
RUN make all GITHUB_TOKEN=${GITHUB_TOKEN} ENVIRONMENT=${ENVIRONMENT}

# Копирование и установка зависимостей
RUN go mod download
COPY . .

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
