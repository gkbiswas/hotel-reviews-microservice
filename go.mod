module github.com/gkbiswas/hotel-reviews-microservice

go 1.23.0

toolchain go1.24.5

require (
	// AWS SDK v2 for S3 operations
	github.com/aws/aws-sdk-go-v2 v1.36.5
	github.com/aws/aws-sdk-go-v2/config v1.29.17
	github.com/aws/aws-sdk-go-v2/credentials v1.17.70
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.17.84
	github.com/aws/aws-sdk-go-v2/service/s3 v1.84.0
	github.com/aws/smithy-go v1.22.4
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0

	// HTTP frameworks and middleware
	github.com/gin-gonic/gin v1.10.1

	// Validation and utilities
	github.com/go-playground/validator/v10 v10.26.0

	// Redis clients (both v8 and v9 for compatibility)
	github.com/go-redis/redis/v8 v8.11.5

	// Authentication and security
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1

	// Database drivers and ORM
	github.com/jackc/pgx/v5 v5.6.0

	// JSON and data processing
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/pkg/errors v0.9.1

	// Metrics and monitoring
	github.com/prometheus/client_golang v1.20.5
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/redis/go-redis/v9 v9.8.0

	// Kafka messaging
	github.com/segmentio/kafka-go v0.4.47

	// Logging
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/pflag v1.0.6 // indirect

	// Configuration management
	github.com/spf13/viper v1.20.1

	// Testing libraries
	github.com/stretchr/testify v1.10.0
	github.com/testcontainers/testcontainers-go v0.38.0
	github.com/testcontainers/testcontainers-go/modules/localstack v0.38.0
	github.com/testcontainers/testcontainers-go/modules/postgres v0.38.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.54.0 // indirect

	// OpenTelemetry and distributed tracing
	go.opentelemetry.io/otel v1.37.0
	go.opentelemetry.io/otel/exporters/jaeger v1.17.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk v1.37.0
	go.opentelemetry.io/otel/trace v1.37.0
	golang.org/x/crypto v0.39.0

	// Synchronization and concurrency
	golang.org/x/sync v0.15.0 // indirect

	// Email and notifications
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
	gopkg.in/yaml.v3 v3.0.1
	gorm.io/driver/postgres v1.6.0
	gorm.io/gorm v1.30.0
)

require (
	// TestContainers indirect dependencies
	dario.cat/mergo v1.0.1 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	// AWS SDK indirect dependencies
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.11 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.32 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.36 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.36 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.36 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.7.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.18.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.30.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.34.0 // indirect

	// Prometheus indirect dependencies
	github.com/beorn7/perks v1.0.1 // indirect

	// Gin and HTTP indirect dependencies
	github.com/bytedance/sonic v1.13.3 // indirect
	github.com/bytedance/sonic/loader v0.2.4 // indirect

	// Redis indirect dependencies
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.5 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v0.2.1 // indirect
	github.com/cpuguy83/dockercfg v0.3.2 // indirect

	// Testing indirect dependencies
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/docker v28.2.2+incompatible // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect

	// System monitoring indirect dependencies
	github.com/ebitengine/purego v0.8.4 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect

	// Validation indirect dependencies
	github.com/gabriel-vasile/mimetype v1.4.9 // indirect
	github.com/gin-contrib/sse v1.1.0 // indirect

	// OpenTelemetry indirect dependencies
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect

	// Configuration management indirect dependencies
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/goccy/go-json v0.10.5 // indirect

	// gRPC and protobuf
	github.com/gogo/protobuf v1.3.2 // indirect

	// Database indirect dependencies
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect

	// Kafka indirect dependencies
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.11 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20240226150601-1dcf7310316a // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/mapstructure v1.5.0
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/go-archive v0.1.0 // indirect
	github.com/moby/patternmatcher v0.6.0 // indirect
	github.com/moby/sys/sequential v0.6.0 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/moby/term v0.5.0 // indirect

	// JSON and utility indirect dependencies
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/sagikazarmark/locafero v0.7.0 // indirect
	github.com/shirou/gopsutil/v4 v4.25.5 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.12.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tklauser/go-sysconf v0.3.13 // indirect
	github.com/tklauser/numcpus v0.7.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect

	// Standard library extensions
	golang.org/x/arch v0.18.0 // indirect
	golang.org/x/mod v0.25.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect

	// Email indirect dependencies
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
)

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/IBM/sarama v1.45.2
	github.com/alicebob/miniredis/v2 v2.35.0
	github.com/confluentinc/confluent-kafka-go v1.9.2
	github.com/golang-migrate/migrate/v4 v4.18.3
	github.com/lib/pq v1.10.9
	golang.org/x/time v0.8.0
	gorm.io/driver/sqlite v1.6.0
)

require (
	github.com/eapache/go-resiliency v1.7.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20230731223053-c322873962e3 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.uber.org/atomic v1.9.0 // indirect
)
