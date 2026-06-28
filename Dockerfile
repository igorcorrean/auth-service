# Estágio 1: Compilação
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copia os arquivos de dependências
COPY go.mod go.sum ./
RUN go mod download

# Copia o resto do código fonte
COPY . .

# Compila o binário de forma estática para produção
RUN CGO_ENABLED=0 GOOS=linux go build -o auth-service .

# Estágio 2: Execução (Imagem final limpa)
FROM alpine:latest

WORKDIR /app

# Copia o binário compilado do estágio anterior
COPY --from=builder /app/auth-service .

# Expõe a porta que a sua aplicação usa
EXPOSE 8001

# Comando para rodar a aplicação
CMD ["./auth-service"]