# syntax=docker/dockerfile:1

FROM golang:1.26-alpine

RUN apk add --no-cache gcc musl-dev geos-dev

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code. Note the slash at the end, as explained in
# https://docs.docker.com/reference/dockerfile/#copy
COPY *.go .

# Build
RUN CGO_ENABLED=1 GOOS=linux go build -v -o /geogo

EXPOSE 9090

CMD ["/geogo"]
