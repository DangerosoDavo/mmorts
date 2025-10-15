module github.com/gravitas-games/mmorts

go 1.21

require (
	github.com/gravitas-015/hexcore v0.0.0-00010101000000-000000000000
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-redis/redis/v8 v8.11.5 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
)

// External packages (local)
replace (
	commandcore => ./external/commandcore
	github.com/DangerosoDavo/ecs => ./external/ecscore
	github.com/DangerosoDavo/udp_network => ./external/udp_network
	github.com/entitycache/entitycache => ./external/cache
	github.com/gravitas-015/hexcore => ./external/hexcore
	github.com/gravitas-015/inventory => ./external/inventory
	github.com/gravitas-015/production => ./external/production
	github.com/mmorts/social => ./external/social
)
