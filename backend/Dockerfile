FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY main.go ./
RUN CGO_ENABLED=0 go build -o /bank-api .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /bank-api ./bank-api
EXPOSE 8080
CMD ["./bank-api"]
