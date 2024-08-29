# Этап сборки
FROM golang:1.22.5-alpine AS builder

# Установка необходимого для сборки
RUN apk add --no-cache git make 

# Установка рабочей директории
WORKDIR /app

COPY go.mod go.sum ./

# Установка переменных окружения как аргументов сборки
ARG GITHUB_TOKEN
ARG ENVIRONMENT
ARG BUILD_TARGET=exchange

# Установка переменных окружения
ENV GOPRIVATE=github.com/sambly
ENV GIT_TERMINAL_PROMPT=1
ENV GITHUB_TOKEN=${GITHUB_TOKEN}
ENV ENVIRONMENT=${ENVIRONMENT}

# Настройка git с использованием переменной GITHUB_TOKEN
RUN git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"

# Установка зависимостей Go
RUN go mod download

# Копируем остальные файлы проекта
COPY internal ./internal
COPY cmd ./cmd

RUN go build -o ./cmd/exchange/myFeederApp ./cmd/${BUILD_TARGET}



# Минимальный финальный образ
FROM alpine:3.18

# Установка зависимостей для выполнения приложения
RUN apk add --no-cache ca-certificates


# Установка зависимостей для выполнения приложения
RUN apk add --no-cache ca-certificates tzdata \
    && ln -sf /usr/share/zoneinfo/Europe/Moscow /etc/localtime \
    && echo "Europe/Moscow" > /etc/timezone


# Устанавливаем рабочую директорию в контейнере
WORKDIR /app/cmd/exchange

COPY --from=builder /app/cmd/exchange/myFeederApp .

# Создание точки монтирования для логов
VOLUME /app/log

# Команда для запуска вашего приложения
CMD ["./myFeederApp"]
