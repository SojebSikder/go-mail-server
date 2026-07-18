# -------- Builder Stage --------
FROM golang:1.25-bookworm AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o myapp main.go

# -------- Final Stage --------
FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /app

COPY --from=builder /app/myapp .

EXPOSE 8080 2525 1430

ENTRYPOINT [ "./myapp" ]
CMD ["start"]
