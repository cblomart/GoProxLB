# GoProxLB Operations Guide

## Overview

GoProxLB is an intelligent load balancer for Proxmox clusters that automatically optimizes VM distribution based on resource usage and your business rules. It prevents node overload, ensures high availability, and maximizes resource utilization.

## Quick Reference

### Essential Commands
```bash
# Start service
goproxlb start --config config.yaml

# Check status
goproxlb status

# Force balancing
goproxlb balance --force

# Install as service
sudo goproxlb install --enable
```

### VM Tagging Rules
| Tag | Purpose | Example |
|-----|---------|---------|
| `plb_affinity_$TAG` | Keep VMs together | `plb_affinity_web` |
| `plb_anti_affinity_$TAG` | Distribute VMs | `plb_anti_affinity_ha` |
| `plb_pin_$NODE` | Pin to node | `plb_pin_node01` |
| `plb_ignore_$TAG` | Exclude from balancing | `plb_ignore_dev` |

## Installation & Setup

### 1. Download Binary
```bash
# Linux AMD64
wget https://github.com/cblomart/GoProxLB/releases/latest/download/goproxlb-linux-amd64
chmod +x goproxlb-linux-amd64
sudo mv goproxlb-linux-amd64 /usr/local/bin/goproxlb

# macOS
wget https://github.com/cblomart/GoProxLB/releases/latest/download/goproxlb-darwin-amd64
chmod +x goproxlb-darwin-amd64
sudo mv goproxlb-darwin-amd64 /usr/local/bin/goproxlb
```

### 2. Create Configuration
```bash
# Create config directory
sudo mkdir -p /etc/goproxlb

# Create configuration file
sudo tee /etc/goproxlb/config.yaml << EOF
proxmox:
  host: "https://your-proxmox:8006"
  token: "your-api-token"
  insecure: false

cluster:
  name: "your-cluster"
  maintenance_nodes: []

balancing:
  enabled: true
  balancer_type: "advanced"
  interval: "5m"
  aggressiveness: "medium"
  cooldown: "2h"
  
  thresholds:
    cpu: 75
    memory: 80
    storage: 85

logging:
  level: "info"
  format: "text"
EOF
```

### 3. Install as Service
```bash
# Install with auto-detection
sudo goproxlb install --enable

# Or with custom config
sudo goproxlb install \
  --config /etc/goproxlb/config.yaml \
  --user goproxlb \
  --group goproxlb \
  --enable
```

## Configuration

### Authentication Options

#### API Token (Recommended)
```yaml
proxmox:
  host: "https://proxmox.example.com:8006"
  token: "admin@pve!goproxlb=your-secure-token"
  insecure: false
```

#### Username/Password
```yaml
proxmox:
  host: "https://proxmox.example.com:8006"
  username: "admin@pve"
  password: "your-password"
  insecure: false
```

#### Local Access (Root)
```yaml
proxmox:
  host: "http://localhost:8006"
  insecure: true
```

### Balancing Configuration

#### Production Settings
```yaml
balancing:
  enabled: true
  balancer_type: "advanced"      # Recommended
  interval: "5m"                 # Check every 5 minutes
  aggressiveness: "medium"       # Balanced approach
  cooldown: "2h"                 # Prevent rapid migrations
  
  thresholds:
    cpu: 75                      # Trigger at 75% CPU
    memory: 80                   # Trigger at 80% memory
    storage: 85                  # Trigger at 85% storage
```

#### Conservative Settings
```yaml
balancing:
  enabled: true
  balancer_type: "threshold"     # Simple threshold-based
  interval: "10m"                # Less frequent checks
  aggressiveness: "low"          # Conservative balancing
  
  thresholds:
    cpu: 85                      # Higher thresholds
    memory: 90
    storage: 95
```

#### Aggressive Settings
```yaml
balancing:
  enabled: true
  balancer_type: "advanced"
  interval: "2m"                 # Very frequent checks
  aggressiveness: "high"         # Aggressive balancing
  cooldown: "30m"                # Shorter cooldown
  
  thresholds:
    cpu: 70                      # Lower thresholds
    memory: 75
    storage: 80
```

### Balancer Types

#### Threshold Balancer
- **When to use**: Small clusters, simple workloads, testing
- **Behavior**: Migrates VMs when nodes exceed thresholds
- **Pros**: Simple, predictable, low overhead
- **Cons**: Reactive, may not optimize long-term

#### Advanced Balancer (Recommended)
- **When to use**: Production environments, large clusters
- **Behavior**: Analyzes historical data for predictive placement
- **Pros**: Proactive, better optimization, prevents flip-flopping
- **Cons**: Higher computational overhead

## VM Placement Rules

### Affinity Rules
Keep related VMs together on the same node:
```bash
# Tag VMs in Proxmox web interface
plb_affinity_web
plb_affinity_database
plb_affinity_app_tier
```

**Example**: Tag `web-server-1`, `web-server-2`, and `load-balancer` with `plb_affinity_web` to ensure they run on the same node.

### Anti-Affinity Rules
Distribute VMs across different nodes for high availability:
```bash
# Tag VMs in Proxmox web interface
plb_anti_affinity_ntp
plb_anti_affinity_ha
plb_anti_affinity_dns
```

**Example**: Tag `ntp-server-1` and `ntp-server-2` with `plb_anti_affinity_ntp` to ensure they run on different nodes.

### VM Pinning
Pin VMs to specific nodes:
```bash
# Tag VMs in Proxmox web interface
plb_pin_node01
plb_pin_node02
plb_pin_storage-node
```

**Example**: Tag a VM with both `plb_pin_node01` and `plb_pin_node02` to allow it to run on either node, with preference for the one with lower resource usage.

### Ignore VMs
Exclude VMs from balancing:
```bash
# Tag VMs in Proxmox web interface
plb_ignore_dev
plb_ignore_test
plb_ignore_backup
```

**Example**: Tag development VMs with `plb_ignore_dev` to prevent them from being moved during balancing operations.

## Operations

### Service Management
```bash
# Check service status
sudo systemctl status goproxlb

# Start service
sudo systemctl start goproxlb

# Stop service
sudo systemctl stop goproxlb

# Restart service
sudo systemctl restart goproxlb

# Enable auto-start
sudo systemctl enable goproxlb

# View logs
sudo journalctl -u goproxlb -f
```

### Monitoring Commands
```bash
# Check balancer status
goproxlb status

# View cluster information
goproxlb cluster

# List VM distribution
goproxlb list

# Show detailed VM information
goproxlb list --detailed

# Show capacity planning
goproxlb capacity

# Show detailed capacity analysis
goproxlb capacity --detailed

# Export capacity data to CSV
goproxlb capacity --csv report.csv
```

### Manual Operations
```bash
# Run one balancing cycle
goproxlb balance

# Force balancing even if no improvement
goproxlb balance --force

# Check Raft cluster status (distributed mode)
goproxlb raft
```

### Maintenance Mode
To put nodes in maintenance mode, add them to the configuration:
```yaml
cluster:
  name: "your-cluster"
  maintenance_nodes: ["node01", "node02"]
```

When a node is in maintenance mode:
- No new VMs will be assigned to it
- Existing VMs will be migrated to other available nodes
- Affinity and anti-affinity rules are respected during migration

## Troubleshooting

### Common Issues

#### Authentication Failed
```bash
# Test API access
curl -k -H "Authorization: PVEAPIToken=token" \
  https://proxmox:8006/api2/json/version

# Check credentials
curl -k -u "username@realm:password" \
  https://proxmox:8006/api2/json/version
```

**Solutions**:
- Verify API token or username/password
- Check SSL certificate (use `insecure: true` for self-signed)
- Ensure user has required permissions

#### No Balancing Actions
```bash
# Check if thresholds are exceeded
goproxlb cluster

# Check VM tags
goproxlb list --detailed

# Force balancing
goproxlb balance --force
```

**Solutions**:
- Verify thresholds are appropriate for your workload
- Check if VMs are tagged with `plb_ignore_*`
- Ensure sufficient nodes are available

#### Migration Failures
```bash
# Check VM status
goproxlb list

# Check node connectivity
ping target-node

# Check storage availability
```

**Solutions**:
- Ensure VMs are running (not stopped/suspended)
- Verify network connectivity between nodes
- Check storage availability on target node
- Verify user permissions for VM migration

#### Service Won't Start
```bash
# Check service logs
sudo journalctl -u goproxlb -n 50

# Test configuration
goproxlb start --config /etc/goproxlb/config.yaml

# Check file permissions
ls -la /etc/goproxlb/
```

**Solutions**:
- Verify configuration file syntax
- Check file permissions and ownership
- Ensure Proxmox API is accessible

### Debug Mode
```bash
# Run with debug logging
goproxlb start --config config.yaml

# Or modify configuration
logging:
  level: "debug"
  format: "text"
```

### Performance Issues
```bash
# Check resource usage
ps aux | grep goproxlb
top -p $(pgrep goproxlb)

# Check memory usage
cat /proc/$(pgrep goproxlb)/status | grep VmRSS
```

**Solutions**:
- Increase balancing interval for large clusters
- Use threshold balancer for simple workloads
- Reduce aggressiveness level

## Performance Tuning

### Resource Usage Guidelines
- **Memory**: 10-50 MB typical usage
- **CPU**: Negligible idle, spikes during balancing
- **Network**: Minimal, only API calls to Proxmox

### Scaling Recommendations
- **Small clusters** (< 10 nodes): 5-minute interval, threshold balancer
- **Medium clusters** (10-50 nodes): 5-minute interval, advanced balancer
- **Large clusters** (50+ nodes): 2-3 minute interval, advanced balancer

### Configuration Tuning
```yaml
# For large clusters
balancing:
  interval: "2m"                 # More frequent checks
  aggressiveness: "high"         # More aggressive balancing
  cooldown: "1h"                 # Shorter cooldown

# For small clusters
balancing:
  interval: "10m"                # Less frequent checks
  aggressiveness: "low"          # Conservative balancing
  cooldown: "4h"                 # Longer cooldown
```

## Security Best Practices

### API Token Security
1. Create dedicated API token for GoProxLB
2. Use minimal required permissions
3. Rotate tokens regularly
4. Store tokens securely (not in version control)

### Network Security
1. Use HTTPS for Proxmox connections
2. Restrict network access with firewalls
3. Use VPN for remote access
4. Monitor API access logs

### Service Security
1. Run as dedicated user (not root)
2. Use systemd security features
3. Restrict file permissions
4. Monitor service logs

## Monitoring Integration

### Prometheus Metrics
```bash
# Check if metrics endpoint is available
curl http://localhost:8080/metrics
```

### Log Aggregation
```bash
# Configure JSON logging for log aggregation
logging:
  level: "info"
  format: "json"
```

### Health Checks
```bash
# Create health check script
#!/bin/bash
if goproxlb status > /dev/null 2>&1; then
    echo "OK"
    exit 0
else
    echo "ERROR"
    exit 1
fi
```

## Backup & Recovery

### Configuration Backup
```bash
# Backup configuration
sudo cp /etc/goproxlb/config.yaml /backup/goproxlb-config-$(date +%Y%m%d).yaml

# Restore configuration
sudo cp /backup/goproxlb-config-20231201.yaml /etc/goproxlb/config.yaml
sudo systemctl restart goproxlb
```

### Service Recovery
```bash
# Reinstall service
sudo goproxlb install --config /etc/goproxlb/config.yaml --enable

# Verify installation
sudo systemctl status goproxlb
goproxlb status
```

## Support

### Getting Help
1. Check this documentation
2. Review configuration examples
3. Enable debug logging
4. Check Proxmox cluster health
5. Verify network connectivity

### Log Locations
- **Systemd logs**: `sudo journalctl -u goproxlb`
- **Service logs**: `/var/log/goproxlb/` (if configured)
- **Application logs**: Console output when running manually

### Common Commands Reference
```bash
# Service management
sudo systemctl {start|stop|restart|status|enable|disable} goproxlb

# Configuration
goproxlb install --config /path/to/config.yaml --enable

# Monitoring
goproxlb {status|cluster|list|capacity|raft}

# Operations
goproxlb {balance|balance --force}

# Troubleshooting
sudo journalctl -u goproxlb -f
goproxlb start --config config.yaml  # Manual run with debug
```
