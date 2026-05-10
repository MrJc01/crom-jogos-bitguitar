#!/bin/bash

# ==============================================================================
# BITGUITAR - LIVE DASHBOARD & MONITOR
# ==============================================================================

# Cores para o terminal (Cyberpunk / Hacker Style)
GREEN='\033[1;32m'
DARK_GREEN='\033[0;32m'
RED='\033[1;31m'
YELLOW='\033[1;33m'
CYAN='\033[1;36m'
NC='\033[0m' # No Color

APP_NAME="bitguitar_server"
PID_FILE=".server.pid"
LOG_FILE="server.log"
PORT="8080"
BOT_PID_FILE=".bot.pid"

# Esconder cursor enquanto o menu roda (limpeza visual)
tput civis
trap 'tput cnorm; exit 0' SIGINT SIGTERM EXIT

# Funções de IP e Rede
get_local_ip() {
    ip route get 1.1.1.1 2>/dev/null | awk '{print $7}' | head -n 1
}

# Verifica status do processo
is_running() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p $PID > /dev/null 2>&1; then
            return 0 # Está rodando
        else
            rm -f "$PID_FILE"
            return 1 # Não rodando
        fi
    else
        return 1 # Não rodando
    fi
}

start_server() {
    if is_running; then
        return
    fi
    go build -o $APP_NAME ./cmd/server
    if [ $? -eq 0 ]; then
        nohup ./$APP_NAME > $LOG_FILE 2>&1 &
        echo $! > $PID_FILE
    fi
}

stop_server() {
    if is_running; then
        PID=$(cat "$PID_FILE")
        kill $PID
        rm -f "$PID_FILE"
    fi
}

start_bots() {
    stop_bots
    tput cnorm # Restaura cursor para digitar
    echo -e "\n${CYAN}--- CONFIGURAÇÃO DE BOTS ---${NC}"
    read -p "Quantos bots deseja simular na rádio? (0 para desligar): " count
    tput civis # Esconde cursor
    if [[ "$count" =~ ^[0-9]+$ ]] && [ "$count" -gt 0 ]; then
        go build -o bitguitar_bot ./cmd/bot
        nohup ./bitguitar_bot $count > bot.log 2>&1 &
        echo $! > $BOT_PID_FILE
    fi
}

stop_bots() {
    pkill -f "bitguitar_bot" 2>/dev/null
    pkill -f "go run ./cmd/bot" 2>/dev/null
    rm -f "$BOT_PID_FILE"
}

# Desenhar Tela Principal (Dashboard Ao Vivo)
draw_dashboard() {
    clear
    LOCAL_IP=$(get_local_ip)
    if [ -z "$LOCAL_IP" ]; then LOCAL_IP="localhost"; fi

    echo -e "${GREEN}╔═════════════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║                    ${CYAN}BITGUITAR SERVER MONITOR - AO VIVO${GREEN}                   ║${NC}"
    echo -e "${GREEN}╠═════════════════════════════════════════════════════════════════════════╣${NC}"

    BOTS_STATUS="${RED}[ DESLIGADO ]${NC}"
    if [ -f "$BOT_PID_FILE" ]; then
        BOT_PID=$(cat "$BOT_PID_FILE")
        if ps -p $BOT_PID > /dev/null 2>&1; then
            BOT_COUNT=$(ps -p $BOT_PID -o args= | awk '{print $2}')
            BOTS_STATUS="${GREEN}[ SIMULANDO ${BOT_COUNT} BOTS ]${NC}"
        else
            rm -f "$BOT_PID_FILE"
        fi
    fi

    if is_running; then
        PID=$(cat "$PID_FILE")
        # Coleta metricas (CPU, MEM, Segundos)
        METRICS=$(ps -p $PID -o %cpu,%mem,etimes --no-headers 2>/dev/null)
        if [ -n "$METRICS" ]; then
            read CPU MEM SECONDS <<< $METRICS
            # Formatar tempo
            H=$((SECONDS / 3600))
            M=$(( (SECONDS % 3600) / 60 ))
            S=$((SECONDS % 60))
            UPTIME=$(printf "%02d:%02d:%02d" $H $M $S)
        else
            CPU="0.0"; MEM="0.0"; UPTIME="00:00:00"
        fi

        echo -e "${GREEN}║ ${YELLOW}STATUS:${NC}    ${GREEN}[ ONLINE ]${NC} (PID: $PID)"
        echo -e "${GREEN}║ ${YELLOW}BOT SWARM:${NC} ${BOTS_STATUS}"
        echo -e "${GREEN}║ ${YELLOW}ADDRESS:${NC}   ${CYAN}http://${LOCAL_IP}:${PORT}${NC}"
        echo -e "${GREEN}║ ${YELLOW}TERMINAL:${NC}  ${DARK_GREEN}curl -sL http://${LOCAL_IP}:${PORT}/play.sh | bash${NC}"
        echo -e "${GREEN}║ ${YELLOW}METRICS:${NC}   CPU: ${CPU}% | RAM: ${MEM}% | UPTIME: ${UPTIME}"
    else
        echo -e "${GREEN}║ ${YELLOW}STATUS:${NC}    ${RED}[ OFFLINE ]${NC}"
        echo -e "${GREEN}║ ${YELLOW}BOT SWARM:${NC} ${BOTS_STATUS}"
        echo -e "${GREEN}║ ${YELLOW}ADDRESS:${NC}   ---"
        echo -e "${GREEN}║ ${YELLOW}TERMINAL:${NC}  ---"
        echo -e "${GREEN}║ ${YELLOW}METRICS:${NC}   N/A"
    fi

    echo -e "${GREEN}╠═════════════════════════════════════════════════════════════════════════╣${NC}"
    echo -e "${GREEN}║ ${CYAN}ÚLTIMOS EVENTOS DE LOG (server.log)${GREEN}                                     ║${NC}"
    echo -e "${GREEN}╟─────────────────────────────────────────────────────────────────────────╢${NC}"
    
    if [ -f "$LOG_FILE" ]; then
        tail -n 6 "$LOG_FILE" | while read -r line; do
            # Trunca a linha para caber visualmente na tela (70 chars)
            printf "${DARK_GREEN}║ %-71s ║${NC}\n" "${line:0:70}"
        done
    else
        echo -e "${DARK_GREEN}║ (Sem logs)                                                              ║${NC}"
    fi

    echo -e "${GREEN}╚═════════════════════════════════════════════════════════════════════════╝${NC}"
    echo -e "${YELLOW}Opções:${NC} [1] Iniciar  [2] Parar  [3] Reiniciar  [B] Bots  [4] Atualizar  [5] Sair"
}

# Processa argumentos se passados
if [ $# -gt 0 ]; then
    case "$1" in
        start) start_server ;;
        stop) stop_server ;;
        restart) stop_server; sleep 1; start_server ;;
        status)
            if is_running; then echo "ONLINE"; else echo "OFFLINE"; fi
            ;;
        *) 
            echo "Uso: $0 {start|stop|restart|status}"
            exit 1 
            ;;
    esac
    exit 0
fi

# Loop principal interativo com Refresh automático a cada 2 segundos
while true; do
    draw_dashboard
    
    # read com timeout de 2 segundos. Se não tiver input, atualiza a tela
    read -t 2 -n 1 -p " > " opt
    
    # Se opt for vazio (timeout), continua o loop (re-desenha a tela)
    if [ -z "$opt" ]; then
        continue
    fi

    # Se digitou algo, processa a acao
    case $opt in
        1) start_server ;;
        2) stop_server ;;
        3) stop_server; sleep 1; start_server ;;
        4) 
            clear; echo -e "${CYAN}Atualizando (go mod tidy)...${NC}"
            go mod tidy
            sleep 2
            ;;
        b|B) start_bots ;;
        5|q|Q) 
            tput cnorm # Restaura cursor
            clear
            echo -e "${GREEN}Saindo do monitor... (O servidor continua rodando se estiver ONLINE)${NC}"
            exit 0 
            ;;
    esac
done
