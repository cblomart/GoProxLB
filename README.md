# GoProxLB - Intelligent Load Balancer for Proxmox

**Automate VM workload distribution across your Proxmox cluster with intelligent, rule-based balancing.**

GoProxLB continuously monitors your Proxmox cluster and automatically migrates VMs to optimize resource utilization, prevent node overload, and maintain high availability.

## 🚀 Quick Start

### 1. Download & Run (5 minutes)
```bash
# Download for your platform
wget https://github.com/cblomart/GoProxLB/releases/latest/download/goproxlb-linux-amd64
chmod +x goproxlb-linux-amd64

# Create minimal config
cat > config.yaml << EOF
proxmox:
  host: "https://your-proxmox:8006"
  token: "your-api-token"
cluster:
  name: "your-cluster"
balancing:
  enabled: true
  interval: "5m"
EOF

# Start balancing
./goproxlb-linux-amd64 start --config config.yaml
```

### 2. Install as Service (Production)
```bash
# Install with auto-detection
sudo ./goproxlb-linux-amd64 install --enable

# Or with custom config
sudo ./goproxlb-linux-amd64 install --config /etc/goproxlb/config.yaml --enable
```

## 🎯 Why GoProxLB?

### **Prevent Node Overload**
- Automatically migrates VMs when nodes exceed 80% CPU/85% memory
- Prevents cascading failures and performance degradation
- Configurable thresholds for your environment

### **High Availability**
- Anti-affinity rules keep critical services on different nodes
- Affinity rules group related VMs for optimal performance
- Automatic failover during node maintenance

### **Resource Optimization**
- Advanced algorithms analyze historical usage patterns
- Predictive placement using P90/P95 percentile analysis
- Intelligent resource distribution based on workload analysis

### **Zero Downtime Operations**
- Live VM migration with no service interruption
- Maintenance mode for planned node work
- Respects VM placement rules during migrations

## 📊 Key Features

| Feature | Description | Benefit |
|---------|-------------|---------|
| **Intelligent Balancing** | Advanced algorithms with historical analysis | Better resource utilization |
| **Rule-Based Placement** | Affinity/anti-affinity rules via VM tags | Predictable, controlled placement |
| **Maintenance Mode** | Automatic VM evacuation from maintenance nodes | Zero-downtime maintenance |
| **Distributed Operation** | Raft-based leader election for HA | No single point of failure |
| **Auto-Detection** | Discovers cluster config automatically | Minimal setup required |
| **Performance Optimized** | CPU/memory optimizations for large clusters | Scales to 100+ nodes |

## 🔧 Configuration Examples

### Basic Production Setup
```yaml
proxmox:
  host: "https://proxmox-cluster.example.com:8006"
  token: "admin@pve!goproxlb=your-secure-token"

cluster:
  name: "production"
  maintenance_nodes: ["node03"]  # Node under maintenance

balancing:
  enabled: true
  balancer_type: "advanced"      # Recommended for production
  interval: "5m"
  aggressiveness: "medium"
  cooldown: "2h"                 # Prevent rapid migrations
  
  # Resource thresholds
  thresholds:
    cpu: 75
    memory: 80
    storage: 85
```

### High Availability Setup
```yaml
balancing:
  # Anti-affinity for critical services
  # Tag VMs with: plb_anti_affinity_web, plb_anti_affinity_db
  
  # Affinity for related services  
  # Tag VMs with: plb_affinity_app_tier
  
  # Pin critical VMs to specific nodes
  # Tag VMs with: plb_pin_node01, plb_pin_node02
```

### Development Environment
```yaml
balancing:
  interval: "10m"                # Less frequent for dev
  aggressiveness: "low"          # Conservative balancing
  thresholds:
    cpu: 60                      # Lower thresholds
    memory: 70

# Tag dev VMs with: plb_ignore_dev
```

## 🏷️ VM Tagging Rules

Control VM placement with simple tags in Proxmox:

| Tag Pattern | Purpose | Example |
|-------------|---------|---------|
| `plb_affinity_$TAG` | Keep VMs together | `plb_affinity_web` |
| `plb_anti_affinity_$TAG` | Distribute VMs | `plb_anti_affinity_ha` |
| `plb_pin_$NODE` | Pin to specific node | `plb_pin_node01` |
| `plb_ignore_$TAG` | Exclude from balancing | `plb_ignore_dev` |

## 📈 Monitoring & Operations

### Check Status
```bash
# Service status
goproxlb status

# Cluster overview
goproxlb cluster

# VM distribution
goproxlb list

# Capacity planning
goproxlb capacity --detailed
```

### Force Balancing
```bash
# Run one balancing cycle
goproxlb balance

# Force balancing even if no improvement
goproxlb balance --force
```

### Service Management
```bash
# Check service status
sudo systemctl status goproxlb

# View logs
sudo journalctl -u goproxlb -f

# Restart service
sudo systemctl restart goproxlb
```

## 🔒 Security & Authentication

### API Token (Recommended)
```yaml
proxmox:
  host: "https://proxmox.example.com:8006"
  token: "admin@pve!goproxlb=your-secure-token"
```

### Username/Password
```yaml
proxmox:
  host: "https://proxmox.example.com:8006"
  username: "admin@pve"
  password: "your-password"
```

### Local Access (Root)
```yaml
proxmox:
  host: "http://localhost:8006"
  insecure: true
```

## 🚀 Deployment Options

### Systemd Service (Recommended)
```bash
# Install with defaults
sudo goproxlb install --enable

# Custom installation
sudo goproxlb install \
  --config /etc/goproxlb/config.yaml \
  --user goproxlb \
  --group goproxlb \
  --enable
```

### Docker
```bash
docker run -d \
  --name goproxlb \
  --restart unless-stopped \
  -v /etc/goproxlb/config.yaml:/app/config.yaml \
  ghcr.io/cedric/goproxlb:latest
```

### Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: goproxlb
spec:
  replicas: 1
  selector:
    matchLabels:
      app: goproxlb
  template:
    metadata:
      labels:
        app: goproxlb
    spec:
      containers:
      - name: goproxlb
        image: ghcr.io/cedric/goproxlb:latest
        volumeMounts:
        - name: config
          mountPath: /app/config.yaml
          subPath: config.yaml
      volumes:
      - name: config
        configMap:
          name: goproxlb-config
```

## 📊 Performance & Scaling

### Resource Usage
- **Memory**: 10-50 MB typical usage
- **CPU**: Negligible idle, spikes during balancing
- **Network**: Minimal, only API calls to Proxmox

### Scaling Considerations
- **Designed for**: Small to medium Proxmox clusters
- **Recommended**: Start with 5-20 nodes for initial deployment
- **Balancing Interval**: 2-10 minutes depending on cluster size
- **Testing needed**: Performance validation for larger clusters

### Performance Optimizations
- Integer math for faster calculations
- Cached time calls to reduce overhead
- Pre-allocated data structures
- Configurable migration limits

## 🔧 Troubleshooting

### Common Issues

**Authentication Failed**
```bash
# Test API access
curl -k -H "Authorization: PVEAPIToken=token" \
  https://proxmox:8006/api2/json/version
```

**No Balancing Actions**
```bash
# Check thresholds
goproxlb cluster

# Check VM tags
goproxlb list --detailed

# Force balancing
goproxlb balance --force
```

**Migration Failures**
```bash
# Check VM status
goproxlb list

# Check node connectivity
ping target-node

# Check storage availability
```

### Debug Mode
```bash
# Run with debug logging
goproxlb start --config config.yaml

# Or modify config
logging:
  level: "debug"
  format: "text"
```

## 📚 Documentation

- **[Usage Guide](docs/USAGE.md)** - Detailed configuration and operation
- **[Deployment Guide](docs/DEPLOYMENT.md)** - Security and deployment options
- **[Configuration Examples](config-examples/)** - Ready-to-use configs

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

**Ready to optimize your Proxmox cluster?** Start with the [Quick Start](#-quick-start) guide above!