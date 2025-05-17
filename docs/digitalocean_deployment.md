# DigitalOcean Deployment Guide

This guide explains how to deploy the Jim.Tennis application to a DigitalOcean droplet using Docker.

## Prerequisites

1. A DigitalOcean account
2. A droplet running Ubuntu (recommended: Ubuntu 20.04 LTS or newer)
3. SSH access to your droplet
4. (Optional) A domain name pointed to your droplet's IP address

## Deployment Options

### Option 1: Automated Deployment Script

We provide a deployment script that handles everything for you:

1. Edit the configuration section in `scripts/deploy-digitalocean.sh`:

```bash
# Configuration - Update these values
DROPLET_IP="your-droplet-ip"  # e.g., "123.456.789.012"
SSH_USER="root"               # or another user with sudo privileges
SSH_KEY_PATH="$HOME/.ssh/id_rsa"  # path to your SSH key
DEPLOY_DIR="/opt/jim-dot-tennis"  # deployment directory on the server
APP_DOMAIN="your-domain.com"  # Optional: Set this if you have a domain
```

2. Run the deployment script:

```bash
./scripts/deploy-digitalocean.sh
```

The script will:
- Install Docker and Docker Compose if needed
- Copy all necessary files to the server
- Configure HTTPS with Caddy if a domain is provided
- Set up automated backups
- Build and start the application

### Option 2: Manual Deployment

If you prefer manual deployment:

1. SSH into your DigitalOcean droplet:

```bash
ssh root@your-droplet-ip
```

2. Install Docker and Docker Compose:

```bash
# Update package index
apt-get update

# Install prerequisites
apt-get install -y apt-transport-https ca-certificates curl software-properties-common

# Add Docker's official GPG key
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -

# Add Docker repository
add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"

# Install Docker
apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io

# Install Docker Compose
curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose
```

3. Create a deployment directory:

```bash
mkdir -p /opt/jim-dot-tennis
cd /opt/jim-dot-tennis
```

4. Copy your project files to the server (from your local machine):

```bash
scp -r docker-compose.yml Dockerfile .dockerignore scripts root@your-droplet-ip:/opt/jim-dot-tennis/
```

5. Build and start the application:

```bash
cd /opt/jim-dot-tennis
docker-compose up -d
```

## Setting Up HTTPS with Caddy

For production deployments, we recommend using HTTPS. The automated script handles this if you provide a domain, but you can also set it up manually:

1. Create a `docker-compose.override.yml` file:

```yaml
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
```

2. Create a `Caddyfile`:

```
your-domain.com {
  reverse_proxy app:8080
}
```

3. Restart your application:

```bash
docker-compose up -d
```

## Managing Your Deployment

### Viewing Logs

```bash
docker-compose logs -f
```

### Stopping the Application

```bash
docker-compose down
```

### Restarting the Application

```bash
docker-compose restart
```

### Updating the Application

1. Push your changes to version control
2. SSH into your droplet
3. Pull the latest changes and restart:

```bash
cd /opt/jim-dot-tennis
git pull
docker-compose up -d --build
```

## Backup Management

The deployment includes an automatic backup system that:

1. Creates daily backups within Docker
2. Exports backups to `/opt/jim-dot-tennis/external-backups`
3. Runs a cron job daily at 3 AM

### Manual Backup

To manually trigger a backup:

```bash
docker exec jim-dot-tennis-backup sh -c 'sqlite3 /data/tennis.db ".backup /backups/tennis-$(date +%Y-%m-%d-%H%M%S)-manual.db"'
```

### Restoring from Backup

```bash
# Stop the application
docker-compose down

# Navigate to the volume mount directory
cd /var/lib/docker/volumes/jim-dot-tennis-data/_data

# Restore the database from backup
cp /opt/jim-dot-tennis/external-backups/tennis-backup-file.db ./tennis.db

# Start the application
docker-compose up -d
```

## Monitoring and Maintenance

For a production deployment, consider adding:

1. **Monitoring**: Set up Prometheus and Grafana for monitoring
2. **Log Management**: Use a log aggregation service like ELK stack
3. **Automated Alerts**: Configure alerts for critical errors
4. **Offsite Backups**: Configure cloud storage for backup exports