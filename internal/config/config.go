package config

import (
	"fmt"
	"os"
	"reflect"

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

var envPrefix = "FEEDER"

func getEnv(key string) (string, bool) {
	fullKey := envPrefix + "_" + key
	return os.LookupEnv(fullKey)
}

func NewConfig() (*Config, error) {

	var hostDb string
	var hostGrpc string
	production := false
	debug := false

	if os.Getenv("ENVIRONMENT") == "docker" {
		var exists bool
		hostDb, exists = getEnv("DB_HOST_DOCKER")
		if !exists {
			return nil, fmt.Errorf("no found DB_HOST_DOCKER")
		}
		hostGrpc, exists = getEnv("GRPC_HOST_DOCKER")
		if !exists {
			return nil, fmt.Errorf("no found GRPC_HOST_DOCKER")
		}

	} else {
		var exists bool

		if err := godotenv.Load(".env"); err != nil {
			return nil, err
		}

		hostDb, exists = getEnv("DB_HOST_LOCAL")
		if !exists {
			return nil, fmt.Errorf("no found DB_HOST_LOCAL")
		}

		hostGrpc, exists = getEnv("GRPC_HOST_LOCAL")
		if !exists {
			return nil, fmt.Errorf("no found GRPC_HOST_LOCAL")
		}
	}

	exchangeType, exists := getEnv("APP_EXCHANGE_TYPE")
	if !exists {
		return nil, fmt.Errorf("no found APP_EXCHANGE_TYPE")
	}
	// DB
	nameDb, exists := getEnv("DB_NAME")
	if !exists {
		return nil, fmt.Errorf("no found DB_NAME")
	}
	passwordDb, exists := getEnv("DB_PASSWORD")
	if !exists {
		return nil, fmt.Errorf("no found DB_PASSWORD")
	}
	portDb, exists := getEnv("DB_PORT")
	if !exists {
		return nil, fmt.Errorf("no found DB_PORT")
	}

	userDb, exists := getEnv("DB_USER")
	if !exists {
		return nil, fmt.Errorf("no found DB_USER")
	}

	productionString, exists := getEnv("LOG_PRODUCTION")
	if !exists {
		return nil, fmt.Errorf("no found LOG_PRODUCTION")
	}
	if productionString == "true" {
		production = true
	}

	debugString, exists := getEnv("LOG_DEBUG")
	if !exists {
		return nil, fmt.Errorf("no found LOG_DEBUG")
	}
	if debugString == "true" {
		debug = true
	}

	grpcPort, exists := getEnv("GRPC_PORT")
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

func PrintConfig(v interface{}, indent string) {
	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	if val.Kind() == reflect.Ptr {
		val = val.Elem()
		typ = typ.Elem()
	}

	if val.Kind() != reflect.Struct {
		fmt.Printf("%s%v\n", indent, val)
		return
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		value := val.Field(i)

		fieldName := field.Name
		yamlTag := field.Tag.Get("yaml")
		if yamlTag != "" {
			fieldName = yamlTag
		}

		fmt.Printf("%s%s: ", indent, fieldName)

		if value.Kind() == reflect.Struct {
			fmt.Println()
			PrintConfig(value.Interface(), indent+"  ")
		} else {
			fmt.Printf("%v\n", value.Interface())
		}
	}
}
