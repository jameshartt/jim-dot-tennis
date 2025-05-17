# DigitalOcean Monitoring and Management

This guide covers monitoring, scaling, and managing your Jim.Tennis application on DigitalOcean.

## Basic Monitoring with DigitalOcean

DigitalOcean provides built-in monitoring tools that you can use to keep track of your droplet:

1. **Droplet Metrics**: From your DigitalOcean dashboard, select your droplet and go to the "Graphs" tab to view:
   - CPU usage
   - Memory usage
   - Disk I/O
   - Network traffic

2. **Alert Policies**: Set up alert policies to be notified when:
   - CPU usage exceeds a threshold (e.g., 80% for 5 minutes)
   - Memory usage exceeds a threshold
   - Disk space runs low

To set up alerts:
1. Go to the DigitalOcean dashboard
2. Click on "Monitoring" in the left sidebar
3. Select "Alerts"
4. Click "Create Alert Policy"

## Advanced Monitoring with Prometheus and Grafana

For more comprehensive monitoring, you can set up Prometheus and Grafana:

1. Create a `docker-compose.override.yml` file that includes Prometheus and Grafana:

```yaml
version: '3.8'

services:
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    restart: unless-stopped
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
      - '--web.enable-lifecycle'
    ports:
      - "9090:9090"

  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    restart: unless-stopped
    volumes:
      - grafana-data:/var/lib/grafana
    ports:
      - "3000:3000"
    depends_on:
      - prometheus

volumes:
  prometheus-data:
    name: jim-dot-tennis-prometheus-data
  grafana-data:
    name: jim-dot-tennis-grafana-data
```

2. Create a simple `prometheus.yml` configuration file:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'caddy'
    static_configs:
      - targets: ['caddy:2019']

  - job_name: 'node'
    static_configs:
      - targets: ['node-exporter:9100']

  - job_name: 'docker'
    static_configs:
      - targets: ['docker-exporter:9323']
```

3. Add node-exporter to monitor the system:

```yaml
node-exporter:
  image: prom/node-exporter:latest
  container_name: node-exporter
  restart: unless-stopped
  volumes:
    - /proc:/host/proc:ro
    - /sys:/host/sys:ro
    - /:/rootfs:ro
  command:
    - '--path.procfs=/host/proc'
    - '--path.sysfs=/host/sys'
    - '--collector.filesystem.ignored-mount-points=^/(sys|proc|dev|host|etc)($$|/)'
  ports:
    - "9100:9100"
```

4. Access Grafana at http://your-droplet-ip:3000 (default credentials: admin/admin)

## Scaling Options

### Vertical Scaling

The simplest way to scale your application on DigitalOcean is to resize your droplet:

1. Power off your droplet (from the DigitalOcean dashboard)
2. Click on "Resize"
3. Select a larger plan with more CPU, RAM, or disk space
4. Apply the changes and power on the droplet

### Horizontal Scaling with Load Balancing

For more advanced scaling:

1. Create multiple droplets running identical copies of your application
2. Set up a DigitalOcean Load Balancer:
   - Go to the DigitalOcean dashboard
   - Click on "Networking" in the left sidebar
   - Select "Load Balancers"
   - Click "Create Load Balancer"
   - Configure forwarding rules (HTTP/HTTPS)
   - Select your droplets
   - Configure health checks

3. Update your DNS to point to the Load Balancer IP instead of individual droplets

## Backups and Snapshots

### Automated Backups with DigitalOcean

DigitalOcean offers built-in backup options:

1. **Droplet Backups**: Enable weekly backups for your droplet:
   - Select your droplet
   - Go to "Backups"
   - Click "Enable Backups"
   - Cost: 20% of the droplet's price

2. **Snapshots**: Create manual snapshots of your droplet:
   - Select your droplet
   - Go to "Snapshots"
   - Click "Take Snapshot"
   - Cost: $0.05 per GB per month

### Database Backup Options

In addition to the built-in backup container, consider:

1. **DigitalOcean Spaces**: Use DigitalOcean's S3-compatible storage for offsite backups:

```bash
# Install AWS CLI
apt-get install -y awscli

# Configure with Spaces credentials
aws configure

# Update the backup script to use Spaces
# Add to scripts/backup-manager.sh:
aws s3 cp "$BACKUP_DIR/$BACKUP_FILENAME.gz" s3://your-space-name/backups/
```

2. **Scheduled DigitalOcean Snapshots**: Create a script to take automatic snapshots:

```bash
#!/bin/bash
# Set up your DigitalOcean API token
TOKEN="your-api-token"
DROPLET_ID="your-droplet-id"
SNAPSHOT_NAME="jim-tennis-$(date +%Y-%m-%d)"

# Create snapshot using DigitalOcean API
curl -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"type\":\"snapshot\",\"name\":\"$SNAPSHOT_NAME\"}" \
  "https://api.digitalocean.com/v2/droplets/$DROPLET_ID/actions"

# Delete old snapshots (older than 7 days)
# This requires additional API calls to list and delete snapshots
```

## Security Best Practices

1. **Firewall Setup**: Configure the DigitalOcean Firewall:
   - Go to "Networking" > "Firewalls"
   - Create a new firewall
   - Allow only necessary ports:
     - SSH (22)
     - HTTP (80)
     - HTTPS (443)
   - Apply to your droplet

2. **SSH Hardening**: Improve SSH security:
   - Disable password authentication
   - Use SSH keys only
   - Consider changing the default SSH port

3. **Updates and Patches**: Keep your system updated:
   - Create a cron job for automatic updates:
     ```bash
     # Add to crontab
     0 2 * * * apt-get update && apt-get upgrade -y
     ```

4. **SSL/TLS**: Ensure your site uses HTTPS (already configured with Caddy)

## Troubleshooting Common Issues

### Application Not Accessible

1. Check if containers are running:
   ```bash
   docker ps
   ```

2. Check firewall settings:
   ```bash
   ufw status
   ```

3. Verify Caddy logs:
   ```bash
   docker logs jim-dot-tennis-caddy
   ```

### High CPU or Memory Usage

1. Identify the resource-intensive container:
   ```bash
   docker stats
   ```

2. Check application logs for issues:
   ```bash
   docker logs jim-dot-tennis
   ```

3. Consider scaling up your droplet if consistently high

### Database Corruption

1. Stop the application:
   ```bash
   docker-compose down
   ```

2. Restore from the latest backup:
   ```bash
   # Find the latest backup
   ls -la /opt/jim-dot-tennis/external-backups
   
   # Restore it
   cp /opt/jim-dot-tennis/external-backups/latest-backup.db /var/lib/docker/volumes/jim-dot-tennis-data/_data/tennis.db
   ```

3. Restart the application:
   ```bash
   docker-compose up -d
   ```