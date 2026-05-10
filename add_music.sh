#!/bin/bash

echo -e "\033[0;32m"
echo "====================================="
echo "   BITGUITAR - ADICIONAR MÚSICA      "
echo "====================================="
echo -e "\033[0m"

if [ -z "$1" ]; then
    echo "Uso: ./add_music.sh <caminho/para/arquivo.mp3>"
    echo "Exemplo: ./add_music.sh ~/Downloads/minha_musica.mp3"
    exit 1
fi

FILE="$1"

if [ ! -f "$FILE" ]; then
    echo "Erro: Arquivo '$FILE' não encontrado."
    exit 1
fi

# Pega o nome do arquivo, converte pra minúsculo, remove espaços e acentos básicos
BASENAME=$(basename "$FILE")
NEW_NAME=$(echo "$BASENAME" | tr '[:upper:]' '[:lower:]' | tr ' ' '_' | sed 's/[^a-z0-9_\.]//g')

DEST="musicas/$NEW_NAME"

echo "Copiando '$FILE' para '$DEST'..."
cp "$FILE" "$DEST"

if [ $? -eq 0 ]; then
    echo -e "\033[0;32m✓ Cópia concluída!\033[0m"
    echo "Para que o servidor carregue a nova música, o serviço precisa ser reiniciado."
    
    # Pergunta se o usuário quer reiniciar
    read -p "Deseja reiniciar o container/servidor agora? (s/N) " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Ss]$ ]]; then
        if command -v docker-compose &> /dev/null; then
            echo "Reiniciando via docker-compose..."
            docker-compose restart
        elif command -v podman-compose &> /dev/null; then
            echo "Reiniciando via podman-compose..."
            podman-compose restart
        else
            echo "Docker Compose não detectado. Tentando reiniciar processo local (monitor.sh ou systemctl)..."
            pkill -f bitguitar_server
            echo "Processo morto. Se você usa systemd, ele deve subir sozinho. Se usa nohup, inicie manualmente."
        fi
        echo -e "\033[0;32m✓ Servidor reiniciado. Música adicionada à rotação!\033[0m"
    else
        echo "Lembre-se de reiniciar o servidor manualmente para carregar a música."
    fi
else
    echo -e "\033[0;31m✗ Erro ao copiar o arquivo.\033[0m"
    exit 1
fi
