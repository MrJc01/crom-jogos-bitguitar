#!/bin/bash

echo -e "\033[0;34m"
echo "========================================="
echo "   BITGUITAR - ATUALIZAÇÃO DE PRODUÇÃO   "
echo "========================================="
echo -e "\033[0m"

echo "1. Baixando atualizações do GitHub..."
git pull origin master

if [ $? -ne 0 ]; then
    echo -e "\033[0;31m✗ Falha ao fazer pull do repositório. Verifique se há conflitos.\033[0m"
    exit 1
fi
echo -e "\033[0;32m✓ Atualizações baixadas com sucesso.\033[0m\n"

echo "2. Reconstruindo e reiniciando os containers..."
if command -v podman-compose &> /dev/null; then
    podman-compose up -d --build
elif command -v docker-compose &> /dev/null; then
    docker-compose up -d --build
else
    echo -e "\033[0;31m✗ Nenhum orquestrador Docker/Podman encontrado.\033[0m"
    exit 1
fi

if [ $? -eq 0 ]; then
    echo -e "\n\033[0;32m✓ Atualização concluída! O servidor BitGuitar já está rodando a nova versão.\033[0m"
else
    echo -e "\n\033[0;31m✗ Ocorreu um erro ao subir o servidor.\033[0m"
fi
