package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ExchangeType string

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
	DebugLog      bool
	ProductionLog bool
}

func NewConfig() (*Config, error) {

	var hostDb string
	var hostGrpc string
	production := false
	debug := false

	if os.Getenv("ENVIRONMENT") == "docker" {
		var exists bool
		hostDb, exists = os.LookupEnv("DB_HOST_DOCKER")
		if !exists {
			return nil, fmt.Errorf("no found DB_HOST_DOCKER")
		}
		hostGrpc, exists = os.LookupEnv("GRPC_HOST_DOCKER")
		if !exists {
			return nil, fmt.Errorf("no found GRPC_HOST_DOCKER")
		}

	} else {
		var exists bool

		if err := godotenv.Load(".env"); err != nil {
			return nil, err
		}

		hostDb, exists = os.LookupEnv("DB_HOST_LOCAL")
		if !exists {
			return nil, fmt.Errorf("no found DB_HOST_LOCAL")
		}

		hostGrpc, exists = os.LookupEnv("GRPC_HOST_LOCAL")
		if !exists {
			return nil, fmt.Errorf("no found GRPC_HOST_LOCAL")
		}
	}

	exchangeType, exists := os.LookupEnv("APP_EXCHANGE_FLOW_TYPE")
	if !exists {
		return nil, fmt.Errorf("no found APP_EXCHANGE_FLOW_TYPE")
	}
	// DB
	nameDb, exists := os.LookupEnv("DB_NAME")
	if !exists {
		return nil, fmt.Errorf("no found DB_NAME")
	}
	passwordDb, exists := os.LookupEnv("DB_PASSWORD")
	if !exists {
		return nil, fmt.Errorf("no found DB_PASSWORD")
	}
	portDb, exists := os.LookupEnv("DB_PORT")
	if !exists {
		return nil, fmt.Errorf("no found DB_PORT")
	}

	userDb, exists := os.LookupEnv("DB_USER")
	if !exists {
		return nil, fmt.Errorf("no found DB_USER")
	}

	productionString, exists := os.LookupEnv("LOG_PRODUCTION")
	if !exists {
		return nil, fmt.Errorf("no found LOG_PRODUCTION")
	}
	if productionString == "true" {
		production = true
	}

	debugString, exists := os.LookupEnv("LOG_DEBUG")
	if !exists {
		return nil, fmt.Errorf("no found LOG_DEBUG")
	}
	if debugString == "true" {
		debug = true
	}

	grpcPort, exists := os.LookupEnv("GRPC_PORT")
	if !exists {
		return nil, fmt.Errorf("no found GRPC_PORT")
	}

	c := &Config{
		NameDb:     nameDb,
		PasswordDb: passwordDb,
		HostDb:     hostDb,
		PortDb:     portDb,
		UserDb:     userDb,
		GrpcHost:   hostGrpc,
		GrpcPort:   grpcPort,

		DebugLog:      debug,
		ProductionLog: production,

		ExchangeType: exchangeType,
	}
	return c, nil
}
