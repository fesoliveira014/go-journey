VERSION 0.8

# Shared module artifacts — service Earthfiles COPY from these targets
# instead of using ../../ relative paths (which exceed the build context).

gen-mod:
    FROM scratch
    COPY gen/go.mod gen/go.sum* /gen/
    SAVE ARTIFACT /gen gen

gen-src:
    FROM scratch
    COPY gen/ /gen/
    SAVE ARTIFACT /gen gen

pkg-auth-mod:
    FROM scratch
    COPY pkg/auth/go.mod pkg/auth/go.sum* /pkg-auth/
    SAVE ARTIFACT /pkg-auth pkg-auth

pkg-auth-src:
    FROM scratch
    COPY pkg/auth/ /pkg-auth/
    SAVE ARTIFACT /pkg-auth pkg-auth

pkg-otel-mod:
    FROM scratch
    COPY pkg/otel/go.mod pkg/otel/go.sum* /pkg-otel/
    SAVE ARTIFACT /pkg-otel pkg-otel

pkg-otel-src:
    FROM scratch
    COPY pkg/otel/ /pkg-otel/
    SAVE ARTIFACT /pkg-otel pkg-otel

golangci-config:
    FROM scratch
    COPY .golangci.yml /
    SAVE ARTIFACT /.golangci.yml

ci:
    BUILD ./services/auth+lint
    BUILD ./services/auth+test
    BUILD ./services/catalog+lint
    BUILD ./services/catalog+test
    BUILD ./services/gateway+lint
    BUILD ./services/gateway+test
    BUILD ./services/reservation+lint
    BUILD ./services/reservation+test
    BUILD ./services/search+lint
    BUILD ./services/search+test

lint:
    BUILD ./services/auth+lint
    BUILD ./services/catalog+lint
    BUILD ./services/gateway+lint
    BUILD ./services/reservation+lint
    BUILD ./services/search+lint

test:
    BUILD ./services/auth+test
    BUILD ./services/catalog+test
    BUILD ./services/gateway+test
    BUILD ./services/reservation+test
    BUILD ./services/search+test

integration-test:
    BUILD ./services/auth+integration-test
    BUILD ./services/catalog+integration-test
    BUILD ./services/reservation+integration-test
    BUILD ./services/search+integration-test

docker:
    BUILD ./services/auth+docker
    BUILD ./services/catalog+docker
    BUILD ./services/gateway+docker
    BUILD ./services/reservation+docker
    BUILD ./services/search+docker
