module github.com/fesoliveira014/library-system/services/auth

go 1.26.1

require (
	github.com/fesoliveira014/library-system/gen v0.0.0
	github.com/fesoliveira014/library-system/pkg/auth v0.0.0-00010101000000-000000000000
	github.com/golang-migrate/migrate/v4 v4.19.1
	github.com/google/uuid v1.6.0
	golang.org/x/crypto v0.46.0
	golang.org/x/oauth2 v0.36.0
	google.golang.org/grpc v1.79.3
	google.golang.org/protobuf v1.36.11
	gorm.io/driver/postgres v1.6.0
	gorm.io/gorm v1.31.1
)

require (
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.6.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
)

replace (
	github.com/fesoliveira014/library-system/gen => ../../gen
	github.com/fesoliveira014/library-system/pkg/auth => ../../pkg/auth
)
