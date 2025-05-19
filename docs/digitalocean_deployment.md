# DigitalOcean Deployment Guide

This guide explains how to deploy the Jim.Tennis application to a DigitalOcean droplet using Docker.

## Prerequisites

1. A DigitalOcean account
2. A droplet running Ubuntu (recommended: Ubuntu 20.04 LTS or newer)
3. SSH access to your droplet
4. (Optional) A domain name pointed to your droplet's IP address

## Deployment

We use a two-part deployment approach to ensure reliability:

1. A server setup script that runs on the droplet to install dependencies
2. A local deployment script that transfers files and configures the application

### Step 1: Configure the Deployment Script

Edit the configuration section at the top of `scripts/deploy-digitalocean.sh`:

```bash
# Configuration - Update these values
DROPLET_IP=""              # Your droplet's IP address
SSH_USER="root"            # SSH user (usually root for initial setup)
SSH_KEY_PATH=""            # Path to your SSH private key (leave empty for default)
DEPLOY_DIR="/opt/jim-dot-tennis" # Deployment directory on the server
APP_DOMAIN=""              # Optional: your domain if you have one
```

Make sure to set at least the `DROPLET_IP` value. The `SSH_KEY_PATH` can be left empty if your SSH key is in the default location.

### Step 2: Run the Deployment Script

Once you've configured the script, simply run:

```bash
./scripts/deploy-digitalocean.sh
```

The deployment script will:

1. Test the SSH connection to your droplet
2. Detect if this is a new deployment or an update
3. For new deployments:
   - Upload and run the server setup script
   - Install Docker, Docker Compose, and other dependencies
   - Configure firewall and security settings
4. Transfer the application files to the server
5. Configure HTTPS with Caddy if a domain is provided
6. Start the application using Docker Compose

## What's Included

The deployment sets up:

- Docker and Docker Compose
- UFW firewall with proper port settings
- Fail2ban for SSH protection
- A dedicated user account for the application
- HTTPS configuration with Caddy (if domain provided)
- Automatic database backups

## Manual Server Setup (Optional)

If you prefer to set up the server manually or want to understand what the server setup script does, you can:

1. SSH into your DigitalOcean droplet:

```bash
ssh root@your-droplet-ip
```

2. Upload the server setup script:

```bash
scp scripts/digitalocean-server-setup.sh root@your-droplet-ip:/tmp/
```

3. Run the server setup script manually:

```bash
ssh root@your-droplet-ip "chmod +x /tmp/digitalocean-server-setup.sh && sudo /tmp/digitalocean-server-setup.sh"
```

## Setting Up HTTPS with Caddy

HTTPS is automatically configured if you provide a domain name in the deployment script. The deployment creates:

1. A `Caddyfile` with your domain configuration
2. A `docker-compose.override.yml` file that adds the Caddy service
3. Proper port mappings and volume configurations

## Managing Your Deployment

### Viewing Logs

```bash
ssh user@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose logs -f"
```

### Stopping the Application

```bash
ssh user@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose down"
```

### Restarting the Application

```bash
ssh user@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose restart"
```

### Updating the Application

Simply run the deployment script again:

```bash
./scripts/deploy-digitalocean.sh
```

The script will detect the existing installation and update only the necessary files.

## Backup Management

The deployment includes an automatic backup system that:

1. Creates daily backups within Docker
2. Exports backups to `/opt/jim-dot-tennis/external-backups`
3. Runs a cron job daily at 3 AM

### Manual Backup

To manually trigger a backup:

```bash
ssh user@your-droplet-ip "cd /opt/jim-dot-tennis && docker exec jim-dot-tennis-backup sh -c 'sqlite3 /data/tennis.db \".backup /backups/tennis-\$(date +%Y-%m-%d-%H%M%S)-manual.db\"'"
```

### Restoring from Backup

```bash
# Stop the application
ssh user@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose down"

# Restore the database from backup
ssh user@your-droplet-ip "cp /opt/jim-dot-tennis/external-backups/tennis-backup-file.db /var/lib/docker/volumes/jim-dot-tennis-data/_data/tennis.db"

# Start the application
ssh user@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose up -d"
```

## Troubleshooting

### Connection Issues

If you're experiencing SSH connection issues:

1. Check that your SSH key is correctly set up in DigitalOcean
2. Verify the droplet's IP address and your network connectivity
3. Try increasing the connection timeout in the deployment script
4. Ensure the SSH port (22) is open in the droplet's firewall

### Docker Compose Errors

If Docker Compose fails to start the application:

1. Check the application logs for specific errors:
   ```bash
   ssh user@your-droplet-ip "cd /opt/jim-dot-tennis && docker-compose logs"
   ```

2. Verify that all required files were transferred correctly
   ```bash
   ssh user@your-droplet-ip "ls -la /opt/jim-dot-tennis"
   ```

### Database Issues

If you're experiencing database problems:

1. Check if the database exists:
   ```bash
   ssh user@your-droplet-ip "docker exec jim-dot-tennis ls -la /app/data"
   ```

2. If the database is missing or corrupted, restore from a backup as described above