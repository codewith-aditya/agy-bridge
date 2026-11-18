FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /agy-bridge ./cmd/agy-bridge

FROM alpine:3.20
COPY --from=build /agy-bridge /usr/local/bin/agy-bridge
EXPOSE 8198
ENTRYPOINT ["agy-bridge"]
