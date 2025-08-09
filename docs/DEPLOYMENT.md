# GoProxLB Deployment Guide

This guide covers deployment scenarios from the simplest to the most complex use cases.

## 📦 Quick Installation

All deployment methods start with downloading the appropriate binary:

### Option A: Latest Version (Recommended)

```bash
# Get the latest version number
LATEST_VERSION=$(curl -s https://api.github.com/repos/cblomart/GoProxLB/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

# AMD64 (most common)
curl -LO https://github.com/cblomart/GoProxLB/releases/latest/download/goproxlb-linux-amd64
curl -LO https://github.com/cblomart/GoProxLB/releases/latest/download/goproxlb-linux-amd64.sha256
sha256sum -c goproxlb-linux-amd64.sha256
chmod +x goproxlb-linux-amd64
sudo mv goproxlb-linux-amd64 /usr/local/bin/goproxlb

# ARM64 (Raspberry Pi, modern ARM servers)
curl -LO https://github.com/cblomart/GoProxLB/releases/latest/download/goproxlb-linux-arm64
curl -LO https://github.com/cblomart/GoProxLB/releases/latest/download/goproxlb-linux-arm64.sha256
sha256sum -c goproxlb-linux-arm64.sha256
chmod +x goproxlb-linux-arm64
sudo mv goproxlb-linux-arm64 /usr/local/bin/goproxlb

# ARM (older ARM devices)
curl -LO https://github.com/cblomart/GoProxLB/releases/latest/download/goproxlb-linux-arm
curl -LO https://github.com/cblomart/GoProxLB/releases/latest/download/goproxlb-linux-arm.sha256
sha256sum -c goproxlb-linux-arm.sha256
chmod +x goproxlb-linux-arm
sudo mv goproxlb-linux-arm /usr/local/bin/goproxlb
```

### Option B: Specific Version (Manual)

If you prefer to download a specific version (e.g., v1.1.0):

```bash
# Replace v1.1.0 with your desired version
VERSION="v1.1.0"

# AMD64
curl -LO https://github.com/cblomart/GoProxLB/releases/download/${VERSION}/goproxlb-linux-amd64
curl -LO https://github.com/cblomart/GoProxLB/releases/download/${VERSION}/goproxlb-linux-amd64.sha256
sha256sum -c goproxlb-linux-amd64.sha256
chmod +x goproxlb-linux-amd64
sudo mv goproxlb-linux-amd64 /usr/local/bin/goproxlb

# ARM64 or ARM: Replace amd64 with arm64 or arm in the filename
```

**Note**: After installation, you can clean up the downloaded files:
```bash
rm -f goproxlb-linux-*.sha256
```

---

## 🟢 Level 1: Single Node Testing (Simplest)

**Use Case**: Testing GoProxLB on a single Proxmox node as root user.

**Prerequisites**: Root access on a Proxmox VE node.

### Configuration

```yaml
# /etc/goproxlb/config.yaml
proxmox:
  host: "http://localhost:8006"  # Local API access
  insecure: true                # Allow HTTP for local access

cluster:
  name: "test-cluster"

balancing:
  enabled: true
  interval: "10m"               # Check every 10 minutes
  
logging:
  level: "info"
  format: "text"
```

### Quick Start

```bash
# Create config directory
sudo mkdir -p /etc/goproxlb

# Create basic config
sudo tee /etc/goproxlb/config.yaml << 'EOF'
proxmox:
  host: "http://localhost:8006"
  insecure: true

cluster:
  name: "test-cluster"

balancing:
  enabled: true
  interval: "10m"
  
logging:
  level: "info"
  format: "text"
EOF

# Test run
sudo goproxlb status --config /etc/goproxlb/config.yaml

# Start balancing
sudo goproxlb start --config /etc/goproxlb/config.yaml
```

---

## 🟡 Level 2: Systemd Service (Production Single Node)

**Use Case**: Production deployment on a Proxmox node with automatic startup.

**Prerequisites**: Level 1 working + systemd knowledge.

### Installation

```bash
# Install service
sudo goproxlb install --config /etc/goproxlb/config.yaml

# Enable and start
sudo systemctl enable goproxlb
sudo systemctl start goproxlb

# Check status
sudo systemctl status goproxlb
sudo journalctl -u goproxlb -f
```

### Advanced Configuration

```yaml
# /etc/goproxlb/config.yaml
proxmox:
  host: "http://localhost:8006"
  insecure: true

cluster:
  name: "production-cluster"

balancing:
  enabled: true
  interval: "5m"
  algorithm: "advanced"         # Use advanced balancing
  
  # Resource thresholds
  thresholds:
    cpu_percent: 80.0
    memory_percent: 85.0
    storage_percent: 90.0

# Enable capacity planning
capacity_planning:
  enabled: true
  forecast_days: 30
  history_days: 90

logging:
  level: "info"
  format: "json"                # Better for log aggregation

# Optional: Raft for future HA
raft:
  enabled: false
```

---

## 🟠 Level 3: Remote Access with API Token (Secure)

**Use Case**: Managing Proxmox from external server with secure authentication.

**Prerequisites**: Level 2 working + Proxmox API token created.

### Create API Token in Proxmox

1. **Web UI**: Datacenter → Permissions → API Tokens
2. **Create Token**: 
   - User: `goproxlb@pve` (create user first)
   - Token ID: `management`
   - Privilege Separation: ✅ enabled
3. **Required Permissions**:
   ```
   /                  - VM.Migrate, VM.Monitor, VM.PowerMgmt
   /nodes            - Sys.Audit
   /storage          - Datastore.AllocateSpace
   ```

### Configuration

```yaml
# /etc/goproxlb/config.yaml
proxmox:
  host: "https://your-proxmox-host:8006"
  token: "goproxlb@pve!management=your-token-secret-here"
  insecure: false               # Use HTTPS with cert verification
  timeout: "30s"

cluster:
  name: "production-cluster"

balancing:
  enabled: true
  interval: "5m"
  algorithm: "advanced"

logging:
  level: "info"
  format: "json"

# Optional: Enable distributed mode for multiple managers
distributed:
  enabled: false
```

### Firewall Requirements

```bash
# On the management server
sudo ufw allow out 8006/tcp    # Proxmox API
sudo ufw allow out 443/tcp     # HTTPS

# On Proxmox nodes (if firewall enabled)
sudo ufw allow 8006/tcp        # API access
```

---

## 🔴 Level 4: Docker Deployment (Containerized)

**Use Case**: Running GoProxLB in containers for better isolation and orchestration.

**Prerequisites**: Level 3 working + Docker installed.

### Single Container Deployment

```bash
# Create config directory
mkdir -p /opt/goproxlb

# Create configuration
cat > /opt/goproxlb/config.yaml << 'EOF'
proxmox:
  host: "https://your-proxmox-host:8006"
  token: "goproxlb@pve!management=your-token-secret"
  insecure: false

cluster:
  name: "docker-managed-cluster"

balancing:
  enabled: true
  interval: "5m"
  algorithm: "advanced"

logging:
  level: "info"
  format: "json"
EOF

# Run container
docker run -d \
  --name goproxlb \
  --restart unless-stopped \
  --memory=512m \
  --cpus=0.5 \
  -v /opt/goproxlb/config.yaml:/app/config.yaml:ro \
  ghcr.io/cblomart/goproxlb:latest
```

### Docker Compose

```yaml
# docker-compose.yml
version: '3.8'

services:
  goproxlb:
    image: ghcr.io/cblomart/goproxlb:latest
    container_name: goproxlb
    restart: unless-stopped
    
    # Resource limits
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: '0.5'
    
    # Configuration
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - goproxlb-data:/app/data
    
    # Networking
    networks:
      - proxmox-mgmt
    
    # Health check
    healthcheck:
      test: ["CMD", "/app/goproxlb", "status", "--config", "/app/config.yaml"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

volumes:
  goproxlb-data:

networks:
  proxmox-mgmt:
    driver: bridge
```

```bash
# Deploy with compose
docker-compose up -d

# Monitor logs
docker-compose logs -f goproxlb
```

---

## 🟣 Level 5: High Availability Multi-Node (Complex)

**Use Case**: Multiple GoProxLB instances in HA configuration with Raft consensus.

**Prerequisites**: Level 4 working + multiple servers + networking knowledge.

### Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   GoProxLB-1    │    │   GoProxLB-2    │    │   GoProxLB-3    │
│    (Leader)     │◄──►│   (Follower)    │◄──►│   (Follower)    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Proxmox Cluster                            │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐                │
│  │   Node 1    │ │   Node 2    │ │   Node 3    │                │
│  └─────────────┘ └─────────────┘ └─────────────┘                │
└─────────────────────────────────────────────────────────────────┘
```

### Configuration (Node 1 - Leader)

```yaml
# /etc/goproxlb/config.yaml
proxmox:
  host: "https://proxmox-cluster.example.com:8006"
  token: "goproxlb@pve!cluster=your-token-secret"
  insecure: false

cluster:
  name: "ha-production-cluster"

balancing:
  enabled: true
  interval: "2m"                # Faster intervals for HA
  algorithm: "advanced"

# High Availability Configuration
distributed:
  enabled: true
  node_id: "goproxlb-1"
  bind_addr: "10.0.1.10:9000"
  
  # Raft cluster configuration
  peers:
    - "goproxlb-1=10.0.1.10:9000"
    - "goproxlb-2=10.0.1.11:9000"
    - "goproxlb-3=10.0.1.12:9000"
  
  # Data persistence
  data_dir: "/var/lib/goproxlb"
  
  # Leadership settings
  election_timeout: "10s"
  heartbeat_timeout: "2s"

logging:
  level: "info"
  format: "json"

# Metrics and monitoring
metrics:
  enabled: true
  listen_addr: "0.0.0.0:8080"
  path: "/metrics"
```

### Configuration (Node 2 & 3 - Followers)

```yaml
# Similar config with different node_id and bind_addr
distributed:
  enabled: true
  node_id: "goproxlb-2"         # or "goproxlb-3"
  bind_addr: "10.0.1.11:9000"   # or "10.0.1.12:9000"
  
  peers:
    - "goproxlb-1=10.0.1.10:9000"
    - "goproxlb-2=10.0.1.11:9000"
    - "goproxlb-3=10.0.1.12:9000"
  # ... rest same as Node 1
```

### Networking Requirements

```bash
# Firewall rules for HA cluster
sudo ufw allow from 10.0.1.0/24 to any port 9000    # Raft communication
sudo ufw allow from 10.0.1.0/24 to any port 8080    # Metrics
sudo ufw allow out 8006/tcp                          # Proxmox API
```

### Deployment with Docker Swarm

```yaml
# docker-stack.yml
version: '3.8'

services:
  goproxlb:
    image: ghcr.io/cblomart/goproxlb:latest
    
    deploy:
      replicas: 3
      placement:
        constraints:
          - node.role == manager
      resources:
        limits:
          memory: 1G
          cpus: '1.0'
      restart_policy:
        condition: on-failure
        delay: 10s
        max_attempts: 3
    
    configs:
      - source: goproxlb-config
        target: /app/config.yaml
    
    volumes:
      - goproxlb-data:/var/lib/goproxlb
    
    networks:
      - goproxlb-cluster
      - proxmox-api
    
    ports:
      - "8080:8080"  # Metrics
      - "9000:9000"  # Raft communication

configs:
  goproxlb-config:
    file: ./config.yaml

volumes:
  goproxlb-data:

networks:
  goproxlb-cluster:
    driver: overlay
    attachable: true
  proxmox-api:
    driver: overlay
    external: true
```

---

## 🔧 Operations & Monitoring

### Health Checks

```bash
# Basic status check
goproxlb status --config /etc/goproxlb/config.yaml

# Detailed cluster status
goproxlb raft-status --config /etc/goproxlb/config.yaml

# Capacity planning
goproxlb capacity-planning --config /etc/goproxlb/config.yaml
```

### Metrics & Monitoring

```bash
# Prometheus metrics (if enabled)
curl http://localhost:8080/metrics

# Common metrics to monitor:
# - goproxlb_cluster_nodes_total
# - goproxlb_balancer_migrations_total
# - goproxlb_raft_leader_status
```

### Log Analysis

```bash
# Systemd logs
sudo journalctl -u goproxlb -f --output=json | jq .

# Docker logs
docker logs -f goproxlb | jq .

# Key log fields to monitor:
# - level: "error" or "warn"
# - msg: contains "migration", "election", "api_error"
```

---

## 🚨 Troubleshooting

### Common Issues by Level

#### Level 1-2 (Local/Systemd)
```bash
# Check Proxmox API
sudo systemctl status pveproxy
curl -k http://localhost:8006/api2/json/version

# Check permissions
sudo -u goproxlb goproxlb status --config /etc/goproxlb/config.yaml
```

#### Level 3 (Remote)
```bash
# Test API token
curl -k -H "Authorization: PVEAPIToken=your-token" \
  https://your-host:8006/api2/json/version

# Check connectivity
telnet your-proxmox-host 8006
```

#### Level 4 (Docker)
```bash
# Container diagnostics
docker exec -it goproxlb goproxlb status --config /app/config.yaml
docker stats goproxlb
docker logs goproxlb | grep -i error
```

#### Level 5 (HA)
```bash
# Raft cluster status
docker exec -it goproxlb goproxlb raft-status --config /app/config.yaml

# Check leader election
docker service logs goproxlb_goproxlb | grep -i "leader\|election"

# Network connectivity test
docker exec -it goproxlb nc -zv goproxlb-peer-ip 9000
```

### Emergency Procedures

```bash
# Safe shutdown
sudo systemctl stop goproxlb          # Systemd
docker-compose down                   # Docker
docker stack rm goproxlb             # Swarm

# Force restart with clean state
sudo rm -rf /var/lib/goproxlb/raft   # Clear raft state
sudo systemctl start goproxlb
```

---

## 📊 Performance Guidelines

| Deployment Level | Recommended Resources | VM Count Limit | Check Interval |
|------------------|----------------------|----------------|----------------|
| Level 1-2        | 512MB RAM, 0.5 CPU   | < 50 VMs       | 10-15 minutes  |
| Level 3          | 1GB RAM, 1 CPU       | < 200 VMs      | 5-10 minutes   |
| Level 4          | 1GB RAM, 1 CPU       | < 500 VMs      | 5 minutes      |
| Level 5          | 2GB RAM, 2 CPU       | < 1000 VMs     | 2-5 minutes    |

---

Choose the deployment level that matches your infrastructure complexity and requirements. Start simple and scale up as needed!