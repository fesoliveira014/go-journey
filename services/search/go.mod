module github.com/fesoliveira014/library-system/services/search

go 1.26.1

replace github.com/fesoliveira014/library-system/gen => ../../gen

replace github.com/fesoliveira014/library-system/pkg/auth => ../../pkg/auth

require github.com/meilisearch/meilisearch-go v0.36.1

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
)
