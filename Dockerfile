# syntax=docker/dockerfile:1.4

# STEP 1 -> Build stage.
FROM golang:1.24-alpine AS builder
WORKDIR /project

RUN apk add --no-cache make git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN make

# STEP 2 -> Create production image.
FROM alpine:3 AS production

WORKDIR /opt/magic/magic-api

RUN mkdir -p config bin _storeinit migrations

# Copy all binaries.
COPY --from=builder \
    /project/bin \
    bin


COPY --from=builder \
    /project/config/magic.yaml \
    /project/config/rbac_model.conf \
    /project/cmd/_storeinit/config.yaml \
    config/


COPY --from=builder \
    /project/migrations \
    /migrations

# Run the binary.
ENTRYPOINT ["bin/magic"]