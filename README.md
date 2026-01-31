# MikroMon - ISP Command Center

Sistema de monitoramento e gerência para Provedores de Internet (ISP), focado em OLTs e MikroTik.

## Estrutura do Projeto

- **Backend**: Go (Golang) com Gorilla Mux.
- **Frontend**: Single Page Application (SPA) HTML5 + TailwindCSS + Xterm.js via CDN.
- **Banco de Dados**: MongoDB (Persistência de equipamentos, usuários, logs).
- **Provedor de Acesso**: Nginx (Reverse Proxy + SSL).

## Funcionalidades Implementadas

1.  **Monitoramento de Sinais**:
    - Densidade de portas PON (/api/olt/stats).
    - Top 10 Piores Sinais (/api/network/critical-signals).
2.  **Gerência de Equipamentos**:
    - Cadastro de OLTs, Roteadores e Switchs.
    - Provisionamento de ONUs (Mock).
3.  **Terminal Web (SSH)**:
    - Conexão via WebSockets.
    - Pool de conexões SSH persistentes.
    - Xterm.js para emulação de terminal completa.
4.  **Automação**:
    - Agendamento de Scripts.
    - Comandos Personalizados.
5.  **Segurança e Infraestrutura**:
    - Autenticação JWT.
    - Graceful Shutdown.
    - Docker Compose com Multi-stage build.
    - Gerenciamento de SSL via Certbot (placeholder config).

## Como Rodar em Produção

### 1. Pré-requisitos
- Docker e Docker Compose instalados.

### 2. Configuração
Copie o exemplo de variáveis de ambiente:
```bash
cp .env.example .env
```
Edite o arquivo `.env` com suas credenciais do MongoDB e domínios.

### 3. Deploy
Execute o comando para subir toda a stack:
```bash
docker-compose up -d --build
```

O sistema estará acessível em:
- HTTP: http://localhost (Redireciona para HTTPS se configurado)
- HTTPS: https://localhost (Requer configuração de certificados)

### 4. Inicialização do Banco de Dados
Para criar os índices e usuário admin inicial:
```bash
# Execute o script de seed
go run scripts/seed_db.go
```
*Ou conecte-se ao container do Mongo e insira manualmente caso não tenha Go instalado no host.*

### 5. Configuração Nginx / SSL
Os arquivos de configuração do Nginx estão em `./nginx/conf.d`.
Para gerar certificados SSL com Certbot:
```bash
docker-compose run --rm certbot certonly --webroot --webroot-path /var/www/certbot -d SEU_DOMINIO
```
Descomente as linhas de SSL em `./nginx/conf.d/default.conf` após gerar o certificado e reinicie o Nginx:
```bash
docker-compose restart micromon-proxy
```

## Credenciais Padrão (Seeding)
- **Usuário**: admin
- **Senha**: admin

## Desenvolvimento
Para rodar localmente sem Docker (apenas App):
```bash
go run cmd/server/main.go
```
Certifique-se de ter um MongoDB rodando localmente ou defina `MONGO_URI`.

---
*Gerado por Antigravity AI Agent*
