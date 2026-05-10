# Multi-stage build para manter a imagem final pequena
FROM golang:1.24-alpine AS builder

# Variáveis de ambiente pro Go
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app

# Copia e baixa dependências
COPY go.mod go.sum ./
RUN go mod download

# Copia código-fonte
COPY . .

# Compila o binário do servidor
RUN go build -o bitguitar_server ./cmd/server

# Imagem final hiper-leve
FROM alpine:3.19

WORKDIR /app

# Adicionar pacotes úteis para debug se necessário
RUN apk --no-cache add ca-certificates tzdata

# Copiar o binário compilado da etapa builder
COPY --from=builder /app/bitguitar_server /app/

# Criar os diretórios base
RUN mkdir -p /app/musicas
RUN mkdir -p /app/public

# Copiar arquivos estáticos e os binários pre-compilados do public
COPY --from=builder /app/public /app/public

# Volume principal que deve ser linkado
VOLUME ["/app/musicas"]

# A porta que o servidor vai escutar
EXPOSE 8080

CMD ["./bitguitar_server"]
