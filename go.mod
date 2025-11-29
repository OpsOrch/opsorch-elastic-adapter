module github.com/opsorch/opsorch-elastic-adapter

go 1.22

require (
	github.com/elastic/go-elasticsearch/v8 v8.11.1
	github.com/opsorch/opsorch-core v0.0.0
)

require github.com/elastic/elastic-transport-go/v8 v8.3.0 // indirect

replace github.com/opsorch/opsorch-core => ../opsorch-core
