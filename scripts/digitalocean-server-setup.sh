#!/bin/bash
set -e

# Script to set up a DigitalOcean droplet for Jim.Tennis application
# This script should be run ON the server after SSH connection is established

# Log all output
exec > >(tee -a "/var/log/jim-tennis-setup.log") 2>&1
echo "[$(date)] Starting server setup..."

# Update system packages
echo "[$(date)] Updating system packages..."
apt-get update
apt-get upgrade -y

# Install required packages
echo "[$(date)] Installing required packages..."
apt-get install -y \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg \
    lsb-release \
    fail2ban \
    ufw \
    git \
    make \
    sqlite3

# Install Docker
if ! command -v docker &> /dev/null; then
    echo "[$(date)] Installing Docker..."
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
    apt-get update
    apt-get install -y docker-ce docker-ce-cli containerd.io
fi

# Install Docker Compose
if ! command -v docker-compose &> /dev/null; then
    echo "[$(date)] Installing Docker Compose..."
    curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
    chmod +x /usr/local/bin/docker-compose
fi

# Setup firewall
echo "[$(date)] Configuring firewall..."
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw allow http
ufw allow https
# Only enable UFW if it's not already enabled to avoid getting locked out
if ! ufw status | grep -q "Status: active"; then
    echo "[$(date)] Enabling UFW..."
    echo "y" | ufw enable
fi

# Configure fail2ban
echo "[$(date)] Configuring fail2ban..."
systemctl enable fail2ban
systemctl start fail2ban

# Create application directory
APP_DIR="/opt/jim-dot-tennis"
echo "[$(date)] Creating application directory at $APP_DIR..."
mkdir -p $APP_DIR
mkdir -p $APP_DIR/external-backups

# Create application user
if ! id -u jimtennis &>/dev/null; then
    echo "[$(date)] Creating application user..."
    useradd -m -s /bin/bash jimtennis
    usermod -aG docker jimtennis
    chown -R jimtennis:jimtennis $APP_DIR
fi

# Setup backup cron job
echo "[$(date)] Setting up backup cron job..."
CRON_JOB="0 3 * * * cd $APP_DIR && ./scripts/backup-manager.sh"
(crontab -l 2>/dev/null | grep -v "backup-manager.sh" ; echo "$CRON_JOB") | crontab -

echo "[$(date)] Server setup completed successfully!"
echo "You can now deploy the application using the deployment script." 