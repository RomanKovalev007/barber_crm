package config

type PostgresConfig struct {
	PGHost     string `env:"POSTGRES_HOST" env-default:"db"`
	PGPort     string `env:"POSTGRES_PORT" env-default:"5432"`
	PGUser     string `env:"POSTGRES_USER" env-default:"postgres"`
	PGPassword string `env:"POSTGRES_PASSWORD" env-default:"postgres"`
	PGName     string `env:"POSTGRES_DB"` 
	PGMaxConns int32 `env:"POSTGRES_MAXCONNS" env-default:"20"`
	PGMinConns int32 `env:"POSTGRES_MINCONNS" env-default:"2"`
	PGMaxConnLifetime int32 `env:"POSTGRES_MAXCONNLIFETIME" env-default:"30"`
	PGMaxConnIdleTime int32 `env:"POSTGRES_MAXCONNIDLE" env-default:"5"`

	DSN string
}

type RedisConfig struct {
	RedisPort     string `env:"REDIS_PORT" env-default:"6379"`
	RedisHost     string `env:"REDIS_HOST" env-default:"localhost"`
	RedisDB       int    `env:"REDIS_DB" env-default:"0"`
	RedisPassword string `env:"REDIS_PASSWORD" env-default:""`
	RedisTtlMinute int `env:"REDIS_TTL_MINUTE" env-default:"43200"`
}

type ClickHouseConfig struct {
	Host     string `env:"CLICKHOUSE_HOST"     env-default:"localhost"`
	Port     string `env:"CLICKHOUSE_PORT"     env-default:"9000"`
	Database string `env:"CLICKHOUSE_DATABASE" env-default:"analytics"`
	Username string `env:"CLICKHOUSE_USERNAME" env-default:"default"`
	Password string `env:"CLICKHOUSE_PASSWORD" env-default:""`
}

type KafkaConfig struct {
	Brokers string `env:"KAFKA_BROKERS" env-default:"localhost:9092"`
}