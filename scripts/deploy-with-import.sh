#!/bin/bash
set -e

# Enhanced deployment script with import tools
DROPLET_IP="144.126.228.64"
SSH_USER="root"
DEPLOY_DIR="/opt/jim-dot-tennis"

echo "========================================================"
echo "Jim.Tennis Enhanced Deployment (with Import Tools)"
echo "========================================================"

# Deploy main application files
echo "📦 Deploying application files..."
rsync -avz --exclude '.git' --exclude '.vscode' --exclude 'tennis.db' \
  --exclude 'node_modules' --exclude '.cursor' --exclude '.DS_Store' \
  -e "ssh" \
  ./ $SSH_USER@$DROPLET_IP:$DEPLOY_DIR/

# Make scripts executable
echo "🔧 Setting permissions..."
ssh $SSH_USER@$DROPLET_IP "chmod +x $DEPLOY_DIR/scripts/*.sh"

# Build the import Docker image
echo "🏗️  Building import tools image..."
ssh $SSH_USER@$DROPLET_IP "cd $DEPLOY_DIR && docker build -f Dockerfile.import -t jim-dot-tennis-import:latest ."

# Start/restart main application
echo "🚀 Starting application..."
ssh $SSH_USER@$DROPLET_IP "cd $DEPLOY_DIR && docker-compose up -d app backup caddy"

echo ""
echo "✅ Deployment completed!"
echo ""
echo "🔧 Setup Instructions:"
echo "1. SSH to your server: ssh $SSH_USER@$DROPLET_IP"
echo "2. Go to app directory: cd $DEPLOY_DIR"
echo "3. Set up credentials: ./scripts/tennis-import.sh setup"
echo "4. Test import: ./scripts/tennis-import.sh run-dry"
echo "5. Run full import: ./scripts/tennis-import.sh run"
echo ""
echo "📝 Quick commands:"
echo "   ./scripts/tennis-import.sh status      # Check credentials"
echo "   ./scripts/tennis-import.sh run-week 5  # Import week 5 only"
echo "   ./scripts/tennis-import.sh run-range 1-5  # Import weeks 1-5" 