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

	if os.Getenv("ENVIRONMENT") == "docker" {
		var exists bool
		hostDb, exists = os.LookupEnv("DB_HOST_Docker")
		if !exists {
			return nil, fmt.Errorf("no .env str DB_HOST_Docker  found")
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

	c := &Config{
		NameDb:     nameDb,
		PasswordDb: passwordDb,
		HostDb:     hostDb,
		PortDb:     portDb,
		UserDb:     userDb,
	}
	return c, nil
}
