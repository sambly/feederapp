# Этап сборки
FROM golang:1.24.3-alpine AS builder

# Установка необходимого для сборки
RUN apk add --no-cache git 

# Установка рабочей директории
WORKDIR /app

COPY go.mod go.sum ./


ARG GITHUB_TOKEN
ENV ENVIRONMENT=docker

# Настройка git с использованием переменной GITHUB_TOKEN
RUN git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"

# Установка зависимостей Go
RUN go mod download

# Копируем остальные файлы проекта
COPY internal ./internal
COPY cmd ./cmd

RUN go build -o /app/fedder-app ./cmd


# Минимальный финальный образ
FROM alpine:3.21

# Установка зависимостей для выполнения приложения
RUN apk add --no-cache ca-certificates tzdata \
    && ln -sf /usr/share/zoneinfo/Europe/Moscow /etc/localtime \
    && echo "Europe/Moscow" > /etc/timezone


# Устанавливаем рабочую директорию в контейнере
WORKDIR /app

COPY --from=builder /app/fedder-app .

# Создание точки монтирования для логов
VOLUME /app/log

# Build arguments and OCI-standard labels for versioning
ARG VERSION=unknown
ARG COMMIT_HASH=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.created=${BUILD_DATE}
LABEL org.opencontainers.image.revision=${COMMIT_HASH}
LABEL org.opencontainers.image.version=${VERSION}
LABEL org.opencontainers.image.title="feeder-app"
LABEL org.opencontainers.image.description="Feeder app"

# Команда для запуска вашего приложения
CMD ["./fedder-app"]
