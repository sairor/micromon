package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

func GetProvisionScriptHandler(w http.ResponseWriter, r *http.Request) {
	// Ensure SSH Key exists
	keyDir := "data"
	pubKeyPath := filepath.Join(keyDir, "id_rsa.pub")
	privKeyPath := filepath.Join(keyDir, "id_rsa")

	if _, err := os.Stat(pubKeyPath); os.IsNotExist(err) {
		// Generate Key
		cmd := exec.Command("ssh-keygen", "-t", "rsa", "-b", "2048", "-f", privKeyPath, "-N", "")
		if err := cmd.Run(); err != nil {
			http.Error(w, "Error generating SSH key", http.StatusInternalServerError)
			return
		}
	}

	pubKey, err := os.ReadFile(pubKeyPath)
	if err != nil {
		http.Error(w, "Error reading SSH public key", http.StatusInternalServerError)
		return
	}

	// Use the host from request to be dynamic
	serverIP := r.Host
	if serverIP == "" {
		serverIP = "192.168.0.233" // Fallback
	}

	script := fmt.Sprintf(`{
    # Cores e Estetica
    :local corSucesso "\1B[32m";
    :local corInfo "\1B[36m";
    :local corErro "\1B[31m";
    :local reset "\1B[0m";

    :put ""
    :put "$corInfo  MMM      MMM  III   CCCCCCC  RRRRRRR    OOOOOO   MMM      MMM   OOOOOO   NNN    NN$reset"
    :put "$corInfo  MMMM    MMMM  III  CCCC      RRR  RRR  OOO  OOO  MMMM    MMMM  OOO  OOO  NNNN   NN$reset"
    :put "$corInfo  MMM MMMM MMM  III  CCC       RRRRRRR   OOO  OOO  MMM MMMM MMM  OOO  OOO  NN NN  NN$reset"
    :put "$corInfo  MMM  MM  MMM  III  CCCC      RRR  RR   OOO  OOO  MMM  MM  MMM  OOO  OOO  NN  NN NN$reset"
    :put "$corInfo  MMM      MMM  III   CCCCCCC  RRR   RR   OOOOOO   MMM      MMM   OOOOOO   NN   NNNN$reset"
    :put "------------------------------------------------------------------------------------------"
    :put "          SISTEMA DE CONFIGURACAO AUTOMATICA - AGENTE DE MONITORIA"
    :put "------------------------------------------------------------------------------------------"
    :put ""

    # Passo 1
    :put " > [1/4] Limpando vestigios de instalacoes anteriores... ";
    /file remove [find name="mikromon_key.pub"];
    :delay 800ms;

    # Passo 2
    :put " > [2/4] Gerando credenciais seguras para o usuario 'mikromon'... ";
    :local pass "M0n!t0r_S3cur3_v6_49_#K7x9P2qW5zR1vL"
    :if ([:len [/user find name="mikromon"]] > 0) do={
        /user remove [find name="mikromon"];
    };
    /user add name="mikromon" group=full password=$pass comment="Monitoria SSH - Micromon";
    :put "$corSucesso   [OK] Usuario criado com sucesso.$reset";
    :delay 800ms;

    # Passo 3
    :put " > [3/4] Sincronizando chave publica com o servidor %s... ";
    :do {
        /tool fetch url="http://%s/api/v1/public-key" dst-path=mikromon_key.pub mode=http;
        :delay 2;
    } on-error={ :put "$corErro   [ERRO] Falha de rede: Servidor inacessivel.$reset" };

    # Passo 4
    :put " > [4/4] Finalizando integracao criptografica... ";
    :if ([:len [/file find name="mikromon_key.pub"]] > 0) do={
        /user ssh-keys import public-key-file=mikromon_key.pub user=mikromon;
        :delay 1;
        /file remove [find name="mikromon_key.pub"];
        :put ""
        :put "$corSucesso ################################################### $reset"
        :put "$corSucesso #      INSTALACAO MICROMON FINALIZADA!            # $reset"
        :put "$corSucesso #  ACESSO SSH VIA CHAVE HABILITADO E SEGURO       # $reset"
        :put "$corSucesso ################################################### $reset"
    } else={
        :put "$corErro !!! ERRO CRITICO: A chave SSH nao foi detectada. !!!$reset"
    };
    :put ""
}`, serverIP, serverIP)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"script": script,
		"pubkey": string(pubKey),
	})
}

func GetPublicKeyHandler(w http.ResponseWriter, r *http.Request) {
	pubKeyPath := filepath.Join("data", "id_rsa.pub")
	pubKey, err := os.ReadFile(pubKeyPath)
	if err != nil {
		http.Error(w, "Public key not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write(pubKey)
}
