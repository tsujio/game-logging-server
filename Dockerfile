  
FROM golang:1.16 AS build

WORKDIR /work

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN go build -mod=readonly -v -o /app

FROM gcr.io/distroless/base

WORKDIR /work
COPY --from=build /app ./app

ENTRYPOINT ["./app"]
