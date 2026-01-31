#!/bin/bash

# Mikromon Portable Export Script

echo ">>> Preparando pacote de distribuição Mikromon..."

# 1. Create Dist Directory
mkdir -p dist/mikromon-deploy

# 2. Copy Key Files
cp docker-compose.yml dist/mikromon-deploy/
cp Dockerfile dist/mikromon-deploy/
cp .env.example dist/mikromon-deploy/.env
cp README.md dist/mikromon-deploy/

# 3. Copy Config Directories (Empty structures)
mkdir -p dist/mikromon-deploy/nginx/conf.d
cp nginx/conf.d/default.conf dist/mikromon-deploy/nginx/conf.d/
mkdir -p dist/mikromon-deploy/certbot/conf
mkdir -p dist/mikromon-deploy/certbot/www
mkdir -p dist/mikromon-deploy/data

# 4. Copy Source Code (Optional, if building on target)
# If user wants to just RUN, they use the image. 
# But let's include source so 'docker-compose build' works on target.
echo ">>> Copiando código fonte..."
cp -r cmd dist/mikromon-deploy/
cp -r internal dist/mikromon-deploy/
cp -r web dist/mikromon-deploy/
cp -r scripts dist/mikromon-deploy/
cp go.mod go.sum dist/mikromon-deploy/

# 5. Create Install Script for Target
cat <<EOF > dist/mikromon-deploy/install.sh
#!/bin/bash
echo "Instalando Mikromon..."
if ! command -v docker &> /dev/null
then
    echo "Docker não encontrado. Instale docker e docker-compose primeiro."
    exit 1
fi

echo "Iniciando Containers..."
docker-compose up -d --build

echo "Aguardando inicialização..."
sleep 10

echo "Inicializando Banco de Dados (Seeding)..."
# Tenta rodar o seed via container
docker-compose exec -T micromon-app /mikromon -seed || echo "Aviso: Seed pode ter falhado ou requer intervenção manual se o binário não suportar flag -seed explicita (usando go run scripts/seed_db.go localmente se tiver go)"

echo "Concluido! Acesse http://localhost"
EOF

chmod +x dist/mikromon-deploy/install.sh

echo ">>> Compactando..."
cd dist
tar -czf mikromon-deploy.tar.gz mikromon-deploy
cd ..

echo ">>> Pronto! O arquivo 'dist/mikromon-deploy.tar.gz' contém tudo o que é necessário."
echo ">>> Copie esse arquivo para o novo servidor e rode: ./install.sh"
