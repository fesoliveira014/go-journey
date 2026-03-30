VERSION 0.8

ci:
    BUILD ./services/auth+lint
    BUILD ./services/auth+test
    BUILD ./services/catalog+lint
    BUILD ./services/catalog+test
    BUILD ./services/gateway+lint
    BUILD ./services/gateway+test
    BUILD ./services/reservation+lint
    BUILD ./services/reservation+test

lint:
    BUILD ./services/auth+lint
    BUILD ./services/catalog+lint
    BUILD ./services/gateway+lint
    BUILD ./services/reservation+lint

test:
    BUILD ./services/auth+test
    BUILD ./services/catalog+test
    BUILD ./services/gateway+test
    BUILD ./services/reservation+test
