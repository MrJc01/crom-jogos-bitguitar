# Deploy na VPS - BitGuitar

Este guia descreve como subir o BitGuitar em uma VPS (Linux) utilizando Docker ou Podman.

## 1. Instalação de Dependências
Certifique-se de que a VPS possui `docker` e `docker-compose` instalados. Caso use o ecossistema Podman da CROM, `podman-compose` funciona de maneira idêntica.

## 2. Clonando e Configurando o Repositório
Na sua VPS:
```bash
git clone https://github.com/SEU_USUARIO/crom-jogos-bitguitar.git
cd crom-jogos-bitguitar
```

Crie o arquivo de ambiente:
```bash
cp .env.example .env
```
(Você pode editar o `.env` se quiser rodar numa porta diferente, o padrão é 8080).

Certifique-se de que o arquivo `rankings.json` existe (ou crie-o vazio com `{}`) para que o Docker possa montá-lo como volume:
```bash
echo "{}" > rankings.json
```

## 3. Subindo o Servidor
Com o Docker Compose, basta rodar:
```bash
docker-compose up -d --build
# ou podman-compose up -d --build
```
O servidor estará rodando em plano de fundo e reiniciará automaticamente com o sistema.

Para verificar os logs:
```bash
docker logs -f bitguitar_server
```

## 4. Expondo para o Mundo
Recomendamos o uso do **Nginx** como proxy reverso para servir na porta 80/443 com certificado SSL (Certbot) ou configurar um **Cloudflare Tunnel** apontando para `localhost:8080`.

## 5. Subindo Músicas
Você pode usar o script facilitador na raiz do projeto:
```bash
./add_music.sh caminho/da/musica.mp3
```
Ele copiará a música, arrumará o nome e reiniciará o container automaticamente.
