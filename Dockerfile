# Dockerfile multi-stage optimizado para Thumbi Core API
# Etapa 1: Compilación basada en golang:1.24-alpine
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Instalar certificados CA, git y tzdata para compilación estática
RUN apk add --no-cache ca-certificates git tzdata

# Cachear capas de dependencias Go
COPY go.mod go.sum* ./
RUN go mod download || true

# Copiar código fuente
COPY . .

# Compilar binario estático sin CGO y con strip de símbolos (-w -s)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/thumbi-api \
    ./cmd/api/main.go

# Etapa 2: Imagen mínima de ejecución en producción (alpine:latest con certificados CA)
FROM alpine:latest

WORKDIR /app

# Instalar certificados CA raíz y zona horaria para llamadas HTTPS seguras (Google/Apple OAuth, APIs externas)
RUN apk --no-cache add ca-certificates tzdata

# Crear usuario y grupo sin privilegios de root para hardening y seguridad de contenedores
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copiar el binario compilado y las migraciones DDL de PostgreSQL/PostGIS
COPY --from=builder /app/thumbi-api /app/thumbi-api
COPY --from=builder /app/migrations /app/migrations

# Crear enlace simbólico para compatibilidad histórica con scripts previos
RUN ln -sf /app/thumbi-api /app/auth-kyc-service

# Ajustar permisos de usuario no privilegiado
RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

# Healthcheck nativo de Docker validando el endpoint /health
HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/thumbi-api"]

