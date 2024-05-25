.env
DB_HOST_Docker=mysql
DB_HOST_Local=127.0.0.1
DB_PORT=3306
DB_USER=
DB_PASSWORD=
DB_NAME=



// локальный запуск
go run .

//Эта команда создает Docker-образ с именем feederapp на основе Dockerfile, находящегося в текущей директории.
docker build -t feederapp .

// Запуск контейнера Docker: Эта команда запускает контейнер из образа feederapp в интерактивном режиме (-it). Флаг --rm удаляет контейнер после его остановки.
docker run --rm -it feederapp

// запуск docker compose c построением   Эта команда запускает службы, определенные в docker-compose.yml, и перед запуском пересобирает образы.
docker-compose up --build 

// запуск docker compose Эта команда запускает службы, определенные в docker-compose.yml, без пересборки образов. Она использует существующие образы
docker-compose up



