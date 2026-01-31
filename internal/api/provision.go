package api

import (
    "encoding/json"
    "fmt"
    "net/http"
    // "mikromon/internal/db"
)

type ProvisionRequest struct {
    IP   string `json:"ip"`
    User string `json:"user"`
    Name string `json:"name"`
}

func ProvisionScriptHandler(w http.ResponseWriter, r *http.Request) {
    var req ProvisionRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid input", http.StatusBadRequest)
        return
    }

    // Mock Server IP detection
    serverIP := r.Host // e.g. localhost:8080
    
    // Generate simple Password
    mikromonPass := "StrongPass123!" // In prod, generate random

    // Construct the Mikrotik Script (Ported from Python)
    // Using Go raw string literals for the script content
    script := fmt.Sprintf(`
    # Cores e Estética
    :local corSucesso "\1B[32m";
    :local corInfo "\1B[36m";
    :local corErro "\1B[31m";
    :local reset "\1B[0m";

    :put ""
    :put "$corInfo  MMM      MMM  III   CCCCCCC  RRRRRRR    OOOOOO   MMM      MMM   OOOOOO   NNN    NN$reset"
    :put "$corInfo  MMMM    MMMM  III  CCCC      RRR  RRR  OOO  OOO  MMMM    MMMM  OOO  OOO  NNNN   NN$reset"
    :put "$corInfo  MNM MMMM MMM  III  CCC       RRRRRRR   OOO  OOO  MMM MMMM MMM  OOO  OOO  NN NN  NN$reset"
    :put "------------------------------------------------------------------------------------------"
    :put "          SISTEMA DE CONFIGURACAO AUTOMATICA - AGENTE DE MONITORIA (GO EDITION)"
    :put "          SERVER: %s"
    :put "------------------------------------------------------------------------------------------"

    # Passo 1
    :put " > [1/2] Limpando ambiente... ";
    /user remove [find name="mikromon"];
    :delay 500ms;

    # Passo 2
    :put " > [2/2] Criando usuario de monitoria... ";
    /user add name="mikromon" group=full password="%s" comment="Monitoria Go";
    :put "$corSucesso   [OK] Usuario 'mikromon' criado.$reset";
    
    # VPN / Certs part omitted in Prototype
    :put ""
    :put "$corSucesso* INSTALACAO CONCLUIDA! *$reset"
    `, serverIP, mikromonPass)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"script": script})
}
