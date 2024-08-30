


# docker 
// В первый раз может потребоваться вход 
docker login

//Эта команда создает Docker-образ на основе Dockerfile, находящегося в текущей директории.
docker build -t sambly/feeder_app .

// Запуск с arg 
docker build --build-arg GITHUB_TOKEN=<your_github_token> --build-arg BUILD_TARGET=<target> -t sambly/feeder_app .

// Запуск контейнера Docker: Эта команда запускает контейнер из образа в интерактивном режиме (-it). Флаг --rm удаляет контейнер после его остановки.
docker run --rm -it sambly/feeder_app

// Отправить docker в docker-hub
docker push sambly/feeder_app:latest

# docker-compose
// запуск docker compose c построением. Эта команда запускает службы, определенные в docker-compose.yml, и перед запуском пересобирает образы. -d запуск в фоновом режиме 
docker-compose up --build -d 

// запуск docker compose Эта команда запускает службы, определенные в docker-compose.yml, без пересборки образов. Она использует существующие образы
docker-compose up -d 

// Удаление контейнеров 
docker-compose down 


# make
// Получение зависимостей
make all
// перед push(для проверки .env)
make lint 

