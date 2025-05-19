#!/bin/bash
set -e

# Jim.Tennis Deployment Script for DigitalOcean

# Configuration - Update these values
DROPLET_IP="144.126.228.64"              # Your droplet's IP address
SSH_USER="root"            # SSH user (usually root for initial setup)
SSH_KEY_PATH=""            # Path to your SSH private key (leave empty for default)
DEPLOY_DIR="/opt/jim-dot-tennis" # Deployment directory on the server
APP_DOMAIN="jim.tennis"              # Optional: your domain if you have one

# Validate configuration
if [ -z "$DROPLET_IP" ]; then
  echo "Error: You must set DROPLET_IP in the script."
  exit 1
fi

# Determine SSH key parameter
if [ -n "$SSH_KEY_PATH" ]; then
  SSH_KEY_PARAM="-i $SSH_KEY_PATH"
else
  SSH_KEY_PARAM=""
fi

# SSH options that can help with connection issues
SSH_OPTS="-o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new -o BatchMode=yes"
SCP_OPTS="-o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new"

echo "========================================================"
echo "Jim.Tennis Deployment to DigitalOcean"
echo "========================================================"
echo "Droplet IP:     $DROPLET_IP"
echo "SSH User:       $SSH_USER"
echo "Deploy Directory: $DEPLOY_DIR"
echo "Domain:         ${APP_DOMAIN:-None}"
echo "========================================================"

# Function to handle SSH commands with proper error handling
function remote_command() {
  echo "Running command: $1"
  if ! ssh $SSH_KEY_PARAM $SSH_OPTS $SSH_USER@$DROPLET_IP "$1"; then
    echo "Error: Command failed. Check the log for details."
    exit 1
  fi
}

# Step 1: Test SSH connection
echo "Testing SSH connection..."
if ! ssh $SSH_KEY_PARAM $SSH_OPTS -q $SSH_USER@$DROPLET_IP exit; then
  echo "Error: Cannot establish SSH connection to $DROPLET_IP"
  echo "Please check your SSH key and that the server is reachable."
  exit 1
fi

# Step 2: Verify if it's a new deployment or update
echo "Checking if this is a new deployment or an update..."
if ssh $SSH_KEY_PARAM $SSH_OPTS $SSH_USER@$DROPLET_IP "[ -d $DEPLOY_DIR/.git ]"; then
  echo "Existing deployment found. Updating..."
  IS_UPDATE=true
else
  echo "No existing deployment found. Setting up new deployment..."
  IS_UPDATE=false

  # Transfer and run the server setup script
  echo "Transferring server setup script..."
  scp $SSH_KEY_PARAM $SCP_OPTS scripts/digitalocean-server-setup.sh $SSH_USER@$DROPLET_IP:/tmp/
  
  echo "Running server setup script..."
  remote_command "chmod +x /tmp/digitalocean-server-setup.sh && sudo /tmp/digitalocean-server-setup.sh"
fi

# Step 3: Transfer application files
echo "Transferring application files..."
if [ "$IS_UPDATE" = true ]; then
  # For updates, we'll use rsync which is more efficient
  # First, ensure rsync is installed on the server
  remote_command "if ! command -v rsync >/dev/null; then apt-get update && apt-get install -y rsync; fi"
  
  # Use rsync to transfer files, excluding git directory and other unnecessary files
  rsync -avz --exclude '.git' --exclude '.vscode' --exclude 'tennis.db' \
    --exclude 'node_modules' --exclude '.cursor' --exclude '.DS_Store' \
    -e "ssh $SSH_KEY_PARAM $SSH_OPTS" \
    ./ $SSH_USER@$DROPLET_IP:$DEPLOY_DIR/
else
  # For new deployments, we'll create a tarball and extract it on the server
  echo "Creating deployment archive..."
  tar --exclude='.git' --exclude='.vscode' --exclude='tennis.db' \
      --exclude='node_modules' --exclude='.cursor' --exclude='.DS_Store' \
      -czf /tmp/jim-tennis-deploy.tar.gz .
  
  echo "Uploading deployment archive..."
  scp $SSH_KEY_PARAM $SCP_OPTS /tmp/jim-tennis-deploy.tar.gz $SSH_USER@$DROPLET_IP:/tmp/
  
  echo "Extracting archive on the server..."
  remote_command "mkdir -p $DEPLOY_DIR && tar -xzf /tmp/jim-tennis-deploy.tar.gz -C $DEPLOY_DIR && rm /tmp/jim-tennis-deploy.tar.gz"
  
  # Cleanup local archive
  rm /tmp/jim-tennis-deploy.tar.gz
fi

# Step 4: Set file permissions
echo "Setting file permissions..."
remote_command "chmod +x $DEPLOY_DIR/scripts/*.sh"
remote_command "chown -R jimtennis:jimtennis $DEPLOY_DIR"

# Step 5: Configure Caddy for HTTPS if domain is specified
if [ -n "$APP_DOMAIN" ]; then
  echo "Setting up Caddy for HTTPS with domain: $APP_DOMAIN..."
  
  # Create Caddyfile
  cat > /tmp/Caddyfile << EOF
$APP_DOMAIN {
  reverse_proxy app:8080 {
    header_up Service-Worker-Allowed {http.response.header.Service-Worker-Allowed}
  }
}
EOF
  
  # Upload Caddyfile
  scp $SSH_KEY_PARAM $SCP_OPTS /tmp/Caddyfile $SSH_USER@$DROPLET_IP:$DEPLOY_DIR/
  rm /tmp/Caddyfile
  
  # Add Caddy service to docker-compose if needed
  # For simplicity, we'll create an override file
  cat > /tmp/docker-compose.override.yml << EOF
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
  caddy-config:
EOF
  
  # Upload docker-compose override
  scp $SSH_KEY_PARAM $SCP_OPTS /tmp/docker-compose.override.yml $SSH_USER@$DROPLET_IP:$DEPLOY_DIR/
  rm /tmp/docker-compose.override.yml
fi

# Step 6: Start the application with Docker Compose
echo "Starting the application..."
remote_command "cd $DEPLOY_DIR && docker-compose pull && docker-compose up -d"

# Step 7: Final checks
echo "Checking if application is running..."
sleep 5 # Give containers a moment to start
remote_command "cd $DEPLOY_DIR && docker-compose ps"

echo "========================================================"
echo "Deployment completed successfully!"
echo "========================================================"

if [ -n "$APP_DOMAIN" ]; then
  echo "Your application is available at: https://$APP_DOMAIN"
else
  echo "Your application is available at: http://$DROPLET_IP:8080"
fi

echo "To check the logs, run:"
echo "ssh $SSH_USER@$DROPLET_IP \"cd $DEPLOY_DIR && docker-compose logs -f\"" 