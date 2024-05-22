# Этап сборки
FROM golang:1.21-alpine3.18 AS builder

# Установка необходимого для сборки
RUN apk add --no-cache git

# Установка рабочей директории
WORKDIR /build

# Копирование модульного файла и скачивание зависимостей
ADD . .
RUN go mod download
RUN go mod tidy

# Сборка приложения
RUN go build -o /myFeederApp

# Минимальный финальный образ
FROM alpine:3.18

# Установка зависимостей для выполнения приложения
RUN apk add --no-cache ca-certificates

# Создание точки монтирования для логов
VOLUME /log

# Копирование исполняемого файла из этапа сборки
COPY --from=builder /myFeederApp /usr/local/bin/myFeederApp

# Определение команды запуска
ENTRYPOINT ["/usr/local/bin/myFeederApp"]
