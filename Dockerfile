# syntax=docker/dockerfile:1
FROM golang:1.18 AS build-stage

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /wa-image-fetcher

FROM gcr.io/distroless/base-debian11 AS build-release-stage

WORKDIR /

COPY --from=build-stage /wa-image-fetcher /wa-image-fetcher

EXPOSE 10000

USER nonroot:nonroot

ENTRYPOINT ["/wa-image-fetcher"]