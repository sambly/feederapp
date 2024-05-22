# Этап сборки
FROM golang:1.21-alpine3.18 AS builder

# Установка необходимого для сборки
RUN apk add --no-cache git

# Установка рабочей директории
WORKDIR /

# Копирование модульного файла и скачивание зависимостей
ADD . .
#COPY go.mod go.sum ./
RUN go mod download
RUN go mod tidy

# Копирование остальных файлов
#COPY . .

# Сборка приложения
RUN go build -o /myFeederApp

# Минимальный финальный образ
FROM alpine:3.18

# Установка зависимостей для выполнения приложения
RUN apk add --no-cache ca-certificates

# Копирование исполняемого файла из этапа сборки
COPY --from=builder /myFeederApp /usr/local/bin/myFeederApp

# Определение команды запуска
#ENTRYPOINT ["/usr/local/bin/myFeederApp"]
ENTRYPOINT ["echo", "This is the entrypoint"]
CMD ["This is the CMD"]