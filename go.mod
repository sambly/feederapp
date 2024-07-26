module github.com/sambly/feederApp

go 1.22.5

require (
	github.com/adshao/go-binance/v2 v2.6.0
	github.com/go-sql-driver/mysql v1.8.1
	github.com/joho/godotenv v1.5.1
	github.com/jpillora/backoff v1.0.0
	golang.org/x/sync v0.7.0

)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/bitly/go-simplejson v0.5.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180228061459-e0a39a4cb421 // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/sambly/exchangeService v0.0.0-00010101000000-000000000000 // indirect
)
replace github.com/sambly/exchangeService => ./external/exchangeService

