# Deployment Guide

## Authentication Scenarios

### 1. Running on Proxmox Node Directly (Root Access)

When running GoProxLB directly on a Proxmox node as root, you can access the local API without authentication.

#### Configuration
```yaml
proxmox:
  host: "http://localhost:8006"  # Local API access
  # No authentication needed when running as root
  insecure: true  # Allow HTTP for local access

cluster:
  name: "your-cluster-name"

balancing:
  enabled: true
  interval: "5m"
```

#### Installation
```bash
# Download binary for your platform
wget https://github.com/cblomart/GoProxLB/releases/latest/download/goproxlb-linux-amd64
chmod +x goproxlb-linux-amd64

# Create configuration
cat > config.yaml << EOF
proxmox:
  host: "http://localhost:8006"
  insecure: true

cluster:
  name: "your-cluster-name"

balancing:
  enabled: true
  interval: "5m"
EOF

# Run as root
sudo ./goproxlb-linux-amd64 start --config config.yaml
```

#### Systemd Service
```bash
# Create service file
sudo tee /etc/systemd/system/goproxlb.service << EOF
[Unit]
Description=GoProxLB Load Balancer
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/goproxlb start --config /etc/goproxlb/config.yaml
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl enable goproxlb
sudo systemctl start goproxlb
```

### 2. Remote Access with Username/Password

For accessing Proxmox from a remote machine or with non-root user.

#### Configuration
```yaml
proxmox:
  host: "https://your-proxmox-host:8006"
  username: "your-username@pve"  # or "your-username@pam"
  password: "your-password"
  insecure: false  # Use HTTPS

cluster:
  name: "your-cluster-name"

balancing:
  enabled: true
  interval: "5m"
```

#### User Permissions
The user needs the following permissions in Proxmox:
- **Datastore.AllocateSpace**: For VM migrations
- **VM.Migrate**: For moving VMs between nodes
- **VM.Monitor**: For reading VM status
- **VM.PowerMgmt**: For VM power operations
- **Sys.Audit**: For reading cluster status

### 3. Remote Access with API Token (Recommended)

Most secure method for remote access.

#### Create API Token in Proxmox
1. Go to Proxmox Web UI
2. Navigate to Datacenter → Permissions → API Tokens
3. Create new token with appropriate permissions
4. Note the token ID and secret

#### Configuration
```yaml
proxmox:
  host: "https://your-proxmox-host:8006"
  token: "your-username@pve!token-name=your-token-secret"
  insecure: false

cluster:
  name: "your-cluster-name"

balancing:
  enabled: true
  interval: "5m"
```

### 4. Docker Deployment

#### Using Docker on Proxmox Node
```bash
# Create configuration
mkdir -p /etc/goproxlb
cat > /etc/goproxlb/config.yaml << EOF
proxmox:
  host: "http://host.docker.internal:8006"
  insecure: true

cluster:
  name: "your-cluster-name"

balancing:
  enabled: true
  interval: "5m"
EOF

# Run container
docker run -d \
  --name goproxlb \
  --restart unless-stopped \
  -v /etc/goproxlb/config.yaml:/app/config.yaml \
  ghcr.io/cedric/goproxlb:latest
```

#### Using Docker for Remote Access
```bash
# Create configuration with authentication
mkdir -p /etc/goproxlb
cat > /etc/goproxlb/config.yaml << EOF
proxmox:
  host: "https://your-proxmox-host:8006"
  token: "your-api-token"
  insecure: false

cluster:
  name: "your-cluster-name"

balancing:
  enabled: true
  interval: "5m"
EOF

# Run container
docker run -d \
  --name goproxlb \
  --restart unless-stopped \
  -v /etc/goproxlb/config.yaml:/app/config.yaml \
  ghcr.io/cedric/goproxlb:latest
```

## Security Considerations

### 1. Local Access (Root)
- **Pros**: No authentication needed, simple setup
- **Cons**: Requires root access, less secure
- **Use Case**: Dedicated Proxmox nodes, internal networks

### 2. Username/Password
- **Pros**: Simple to set up, familiar
- **Cons**: Passwords can be compromised, no fine-grained permissions
- **Use Case**: Development, testing environments

### 3. API Token
- **Pros**: Most secure, fine-grained permissions, can be revoked
- **Cons**: More complex setup
- **Use Case**: Production environments, security-conscious deployments

## Network Access

### Firewall Requirements
```bash
# Allow Proxmox API access
sudo ufw allow 8006/tcp

# If using HTTPS
sudo ufw allow 443/tcp

# For cluster communication
sudo ufw allow 3121/tcp  # Proxmox cluster communication
```

### SSL/TLS Configuration
```yaml
# For self-signed certificates
proxmox:
  host: "https://your-proxmox-host:8006"
  token: "your-api-token"
  insecure: true  # Skip certificate verification

# For valid certificates
proxmox:
  host: "https://your-proxmox-host:8006"
  token: "your-api-token"
  insecure: false  # Verify certificates
```

## Monitoring and Logging

### Log Configuration
```yaml
logging:
  level: "info"    # debug, info, warn, error
  format: "json"   # json or text
```

### Systemd Logs
```bash
# View logs
sudo journalctl -u goproxlb -f

# View recent logs
sudo journalctl -u goproxlb --since "1 hour ago"
```

### Docker Logs
```bash
# View logs
docker logs -f goproxlb

# View recent logs
docker logs --since "1h" goproxlb
```

## Troubleshooting

### Common Issues

1. **Authentication Failed**
   ```bash
   # Check credentials
   curl -k -u "username@realm:password" https://your-proxmox-host:8006/api2/json/version
   
   # Check API token
   curl -k -H "Authorization: PVEAPIToken=token" https://your-proxmox-host:8006/api2/json/version
   ```

2. **Connection Refused**
   ```bash
   # Check if Proxmox API is running
   sudo systemctl status pveproxy
   
   # Check firewall
   sudo ufw status
   ```

3. **Permission Denied**
   ```bash
   # Check user permissions in Proxmox
   # Go to Datacenter → Permissions → Users
   # Verify the user has required permissions
   ```

4. **SSL Certificate Issues**
   ```yaml
   # Use insecure mode for self-signed certificates
   proxmox:
     host: "https://your-proxmox-host:8006"
     insecure: true
   ```

### Debug Mode
```bash
# Run with debug logging
./goproxlb start --config config.yaml

# Or modify config
logging:
  level: "debug"
  format: "text"
```

## Performance Tuning

### Resource Limits
```bash
# For systemd service
sudo tee /etc/systemd/system/goproxlb.service << EOF
[Unit]
Description=GoProxLB Load Balancer
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/goproxlb start --config /etc/goproxlb/config.yaml
Restart=always
RestartSec=10
LimitNOFILE=65536
MemoryMax=512M
CPUQuota=50%

[Install]
WantedBy=multi-user.target
EOF
```

### Docker Resource Limits
```bash
docker run -d \
  --name goproxlb \
  --restart unless-stopped \
  --memory=512m \
  --cpus=0.5 \
  -v /etc/goproxlb/config.yaml:/app/config.yaml \
  ghcr.io/cedric/goproxlb:latest
```
