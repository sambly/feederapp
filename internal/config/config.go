package config

import (
	"fmt"
	"os"
	"regexp"

	"github.com/joho/godotenv"
)

type Config struct {
	// DB
	NameDb     string
	PasswordDb string
	HostDb     string
	PortDb     string
	UserDb     string

	// GRPC
	GrpcHost string
	GrpcPort string

	// Log
	Debug      bool
	Production bool
}

func loadEnv(projectDirName string) error {
	projectName := regexp.MustCompile(`^(.*` + projectDirName + `)`)
	currentWorkDirectory, _ := os.Getwd()
	rootPath := projectName.Find([]byte(currentWorkDirectory))
	err := godotenv.Load(string(rootPath) + `/.env`)
	if err != nil {
		return fmt.Errorf("error loading .env file")
	}
	return nil
}

func NewConfig() (*Config, error) {

	var hostDb string
	var hostGrpc string
	production := false
	debug := false

	if os.Getenv("ENVIRONMENT") == "docker" {
		var exists bool
		hostDb, exists = os.LookupEnv("DB_HOST_Docker")
		if !exists {
			return nil, fmt.Errorf("no .env str DB_HOST_Docker  found")
		}
		hostGrpc, exists = os.LookupEnv("grpc_Host_Docker")
		if !exists {
			return nil, fmt.Errorf("no .env str grpc_Host_Docker  found")
		}

	} else {
		var exists bool
		if err := loadEnv("feederApp"); err != nil {
			return nil, err
		}
		hostDb, exists = os.LookupEnv("DB_HOST_Local")
		if !exists {
			return nil, fmt.Errorf("no .env str DB_HOST_Local  found")
		}

		hostGrpc, exists = os.LookupEnv("grpc_Host_Local")
		if !exists {
			return nil, fmt.Errorf("no .env str grpc_Host_Local  found")
		}

		fmt.Println(hostGrpc)
	}
	// DB
	nameDb, exists := os.LookupEnv("DB_NAME")
	if !exists {
		return nil, fmt.Errorf("no .env str DB_NAME found")
	}
	passwordDb, exists := os.LookupEnv("DB_PASSWORD")
	if !exists {
		return nil, fmt.Errorf("no .env str DB_PASSWORD found")
	}
	portDb, exists := os.LookupEnv("DB_PORT")
	if !exists {
		return nil, fmt.Errorf("no .env str DB_PORT found")
	}

	userDb, exists := os.LookupEnv("DB_USER")
	if !exists {
		return nil, fmt.Errorf("no .env str DB_USER found")
	}

	productionString, exists := os.LookupEnv("production")
	if !exists {
		return nil, fmt.Errorf("no .env str production found")
	}
	if productionString == "true" {
		production = true
	}

	debugString, exists := os.LookupEnv("debug")
	if !exists {
		return nil, fmt.Errorf("no .env str debug found")
	}
	if debugString == "true" {
		debug = true
	}

	grpcPort, exists := os.LookupEnv("grpc_Port")
	if !exists {
		return nil, fmt.Errorf("no .env str grpc_Port  found")
	}

	c := &Config{
		NameDb:     nameDb,
		PasswordDb: passwordDb,
		HostDb:     hostDb,
		PortDb:     portDb,
		UserDb:     userDb,
		GrpcHost:   hostGrpc,
		GrpcPort:   grpcPort,

		Debug:      debug,
		Production: production,
	}
	return c, nil
}
