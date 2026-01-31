#!/bin/bash

# Diretório base
BASE_DIR="$HOME/Antigravity"

for port in {8081..8089}
do
    # Define o nome da pasta para cada porta
    DIR_NAME="$BASE_DIR/servidor_$port"
    
    # Cria a pasta se ela não existir
    mkdir -p "$DIR_NAME"
    
    # Cria um arquivo index.html simples se não existir (para teste)
    if [ ! -f "$DIR_NAME/index.html" ]; then
        echo "<h1>Servidor porta $port rodando no Ubuntu</h1>" > "$DIR_NAME/index.html"
    fi
    
    # Inicia o servidor em background
    # O comando cd garante que o servidor inicie dentro da pasta correta
    (cd "$DIR_NAME" && nohup python3 -m http.server $port > /dev/null 2>&1 &)
    
    echo "Iniciado: Porta $port -> $DIR_NAME"
done
