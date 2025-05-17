#!/bin/bash
# DigitalOcean Deployment Script for Jim.Tennis
# This script deploys the application to a DigitalOcean droplet

# Configuration - Update these values
DROPLET_IP=""
SSH_USER="root"
SSH_KEY_PATH="$HOME/.ssh/id_rsa"
DEPLOY_DIR="/opt/jim-dot-tennis"
APP_DOMAIN=""  # Optional: Your domain name

# Colors for pretty output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check if required variables are set
if [ -z "$DROPLET_IP" ]; then
  echo -e "${RED}Error: DROPLET_IP is not set. Please update the script.${NC}"
  exit 1
fi

# Functions
function ssh_command {
  ssh -i "$SSH_KEY_PATH" "${SSH_USER}@${DROPLET_IP}" "$1"
}

function scp_file {
  scp -i "$SSH_KEY_PATH" "$1" "${SSH_USER}@${DROPLET_IP}:$2"
}

# Display info
echo -e "${GREEN}Deploying Jim.Tennis to DigitalOcean...${NC}"
echo "Droplet IP: $DROPLET_IP"
echo "Deploy directory: $DEPLOY_DIR"
echo ""

# 1. Test SSH connection
echo -e "${YELLOW}Testing SSH connection...${NC}"
if ! ssh_command "echo 'SSH connection successful!'"; then
  echo -e "${RED}SSH connection failed. Please check your credentials and ensure the droplet is running.${NC}"
  exit 1
fi

# 2. Set up Docker if not installed
echo -e "${YELLOW}Checking Docker installation...${NC}"
if ! ssh_command "which docker > /dev/null"; then
  echo -e "${YELLOW}Docker not found. Installing Docker...${NC}"
  ssh_command "apt-get update && apt-get install -y apt-transport-https ca-certificates curl software-properties-common"
  ssh_command "curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -"
  ssh_command "add-apt-repository \"deb [arch=amd64] https://download.docker.com/linux/ubuntu \$(lsb_release -cs) stable\""
  ssh_command "apt-get update && apt-get install -y docker-ce docker-ce-cli containerd.io"
  
  # Start Docker service
  ssh_command "systemctl enable docker && systemctl start docker"
  
  # Install Docker Compose
  ssh_command "curl -L \"https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)\" -o /usr/local/bin/docker-compose"
  ssh_command "chmod +x /usr/local/bin/docker-compose"
  
  echo -e "${GREEN}Docker and Docker Compose installed successfully.${NC}"
else
  echo -e "${GREEN}Docker already installed.${NC}"
fi

# 3. Create deployment directory
echo -e "${YELLOW}Setting up deployment directory...${NC}"
ssh_command "mkdir -p $DEPLOY_DIR"

# 4. Copy required files to the server
echo -e "${YELLOW}Copying files to the server...${NC}"
# Create a temporary directory for files
TEMP_DIR=$(mktemp -d)
mkdir -p "$TEMP_DIR/scripts"

# Copy required files to temp directory
cp docker-compose.yml Dockerfile .dockerignore "$TEMP_DIR/"
cp scripts/backup-manager.sh "$TEMP_DIR/scripts/"
chmod +x "$TEMP_DIR/scripts/backup-manager.sh"

# Create .env file for environment variables
cat > "$TEMP_DIR/.env" << EOF
PORT=8080
DB_TYPE=sqlite3
DB_PATH=/app/data/tennis.db
EOF

# If domain is set, create Caddy config for HTTPS
if [ ! -z "$APP_DOMAIN" ]; then
  # Create docker-compose.override.yml with Caddy container
  cat > "$TEMP_DIR/docker-compose.override.yml" << EOF
version: '3.8'

services:
  caddy:
    image: caddy:2-alpine
    container_name: jim-dot-tennis-caddy
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - caddy-data:/data
      - caddy-config:/config
      - ./Caddyfile:/etc/caddy/Caddyfile
    depends_on:
      - app

volumes:
  caddy-data:
    name: jim-dot-tennis-caddy-data
  caddy-config:
    name: jim-dot-tennis-caddy-config
EOF

  # Create Caddyfile
  cat > "$TEMP_DIR/Caddyfile" << EOF
${APP_DOMAIN} {
  reverse_proxy app:8080
}
EOF
  
  # Update app port in docker-compose if using Caddy
  sed -i 's/- "8080:8080"/# - "8080:8080" # Commented out, using Caddy/' "$TEMP_DIR/docker-compose.yml"
  
  echo -e "${GREEN}Created HTTPS proxy configuration with Caddy for ${APP_DOMAIN}${NC}"
fi

# Copy all files to the server
tar -czf "$TEMP_DIR/deploy.tar.gz" -C "$TEMP_DIR" .
scp_file "$TEMP_DIR/deploy.tar.gz" "$DEPLOY_DIR/"
ssh_command "tar -xzf $DEPLOY_DIR/deploy.tar.gz -C $DEPLOY_DIR && rm $DEPLOY_DIR/deploy.tar.gz"

# Clean up temp directory
rm -rf "$TEMP_DIR"

# 5. Configure backup script
echo -e "${YELLOW}Configuring backup script...${NC}"
ssh_command "sed -i 's|/path/to/external/backups|${DEPLOY_DIR}/external-backups|g' ${DEPLOY_DIR}/scripts/backup-manager.sh"
ssh_command "mkdir -p ${DEPLOY_DIR}/external-backups"

# 6. Set up cron job for backups
echo -e "${YELLOW}Setting up backup cron job...${NC}"
ssh_command "(crontab -l 2>/dev/null || echo '') | grep -v '${DEPLOY_DIR}/scripts/backup-manager.sh' | { cat; echo '0 3 * * * ${DEPLOY_DIR}/scripts/backup-manager.sh'; } | crontab -"

# 7. Build and start the application
echo -e "${YELLOW}Building and starting the application...${NC}"
ssh_command "cd $DEPLOY_DIR && docker-compose build && docker-compose up -d"

# 8. Display info about the deployment
echo -e "${GREEN}Deployment completed successfully!${NC}"
echo -e "Your application is now running on the DigitalOcean droplet."

if [ ! -z "$APP_DOMAIN" ]; then
  echo -e "You can access it at: https://${APP_DOMAIN}"
else
  echo -e "You can access it at: http://${DROPLET_IP}:8080"
  echo -e "${YELLOW}Note: For production, consider setting up a domain name and HTTPS.${NC}"
fi

echo -e "\nUseful commands:"
echo -e "- View logs: ssh ${SSH_USER}@${DROPLET_IP} \"cd ${DEPLOY_DIR} && docker-compose logs -f\""
echo -e "- Stop the app: ssh ${SSH_USER}@${DROPLET_IP} \"cd ${DEPLOY_DIR} && docker-compose down\""
echo -e "- Restart the app: ssh ${SSH_USER}@${DROPLET_IP} \"cd ${DEPLOY_DIR} && docker-compose restart\""
echo -e "- Manually trigger backup: ssh ${SSH_USER}@${DROPLET_IP} \"cd ${DEPLOY_DIR} && docker exec jim-dot-tennis-backup sh -c 'sqlite3 /data/tennis.db \".backup /backups/tennis-\$(date +%Y-%m-%d-%H%M%S)-manual.db\"'\""