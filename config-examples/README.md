# GoProxLB Configuration Examples

This directory contains example configuration files for different deployment scenarios. All examples follow the **"Trust must be earned"** philosophy with conservative defaults.

## 🎯 **MLP (Most Lovable Product) Approach**

GoProxLB uses **advanced balancing by default** with **low aggressiveness** to earn your trust:

- ✅ **Advanced balancing** - Maintains even load distribution across nodes
- ✅ **Low aggressiveness** - Conservative by default, won't move VMs unnecessarily  
- ✅ **Sensible defaults** - Minimal configuration required
- ✅ **Progressive trust** - Start conservative, tune after trust is earned
- ✅ **Always enabled** - If you run it, you want balancing (no "enabled" option)
- ✅ **Auto-detection** - Cluster name auto-detected from Proxmox API
- ✅ **HTTPS by default** - Proxmox always uses HTTPS, insecure=true for localhost
- ✅ **Zero configuration** - Works without any config file!

## 🚀 **Zero Configuration Mode**

**GoProxLB works without any configuration file!**

```bash
# Start with zero configuration - auto-detects everything!
goproxlb

# List VMs without config file
goproxlb list

# Show cluster info without config file
goproxlb cluster

# Show capacity planning without config file
goproxlb capacity
```

**What gets auto-detected:**
- ✅ **Proxmox host** - Defaults to `https://localhost:8006`
- ✅ **Cluster name** - Auto-detected from Proxmox API
- ✅ **Balancer type** - Advanced balancing by default
- ✅ **Aggressiveness** - Low (4h cooldown) by default
- ✅ **All other settings** - Sensible defaults for everything

## 🔄 **Cooldown Behavior**

**Cooldown is per-VM, not global:** "Don't touch this VM because we already moved it less than X ago"

| Aggressiveness | Cooldown | Behavior |
|----------------|----------|----------|
| **Low** | 4 hours | Very conservative - won't move VMs frequently |
| **Medium** | 2 hours | Balanced - reasonable stability |
| **High** | 30 minutes | Aggressive - quick response to load changes |

## 📁 **Configuration Examples**

### **1. Zero Configuration (Recommended)**
```bash
# No config file needed!
goproxlb
```

**Use for:** Quick start, testing, development, most use cases

### **2. `simple-advanced.yaml` - Minimal Configuration**
```yaml
# Just the essentials - everything else uses sensible defaults
balancing:
  balancer_type: "advanced"  # Advanced balancing
  aggressiveness: "low"      # Conservative by default (4h cooldown)
```

**Use for:** When you want to be explicit about settings

### **3. `local-development.yaml` - Development Environment**
```yaml
# Development-friendly with debug logging
balancing:
  balancer_type: "advanced"
  aggressiveness: "low"      # 4h cooldown

logging:
  level: "debug"  # Verbose logging for development
```

**Use for:** Local development, testing, debugging

### **4. `production-distributed.yaml` - Production Cluster**
```yaml
# Production-ready with distributed mode
balancing:
  balancer_type: "advanced"
  aggressiveness: "low"  # Start conservative (4h cooldown)

raft:
  enabled: true  # Distributed mode
  auto_discover: true  # Auto-discover peers
```

**Use for:** Production environments, high availability

### **5. `threshold-balancer.yaml` - Simple Threshold Mode**
```yaml
# Basic threshold-based balancing (legacy)
balancing:
  balancer_type: "threshold"  # Simple threshold mode
  thresholds:
    cpu: 80
    memory: 85
```

**Use for:** Simple environments, legacy compatibility

### **6. `high-aggressiveness.yaml` - After Trust is Earned**
```yaml
# More aggressive balancing (use after trust is earned)
balancing:
  balancer_type: "advanced"
  aggressiveness: "high"  # More aggressive (30m cooldown)
```

**Use for:** High-performance environments, after proving reliability

## 🚀 **Quick Start**

### **Zero Configuration (Recommended)**
```bash
# Start with zero configuration - auto-detects everything!
goproxlb

# List VMs
goproxlb list

# Show cluster info
goproxlb cluster

# Show capacity planning
goproxlb capacity
```

### **With Configuration File**
```bash
# Copy the simple example:
cp config-examples/simple-advanced.yaml config.yaml

# Edit Proxmox connection (only needed for remote):
proxmox:
  host: "https://your-proxmox-host:8006"
  username: "your-username"
  password: "your-password"

# Start GoProxLB:
goproxlb --config config.yaml
```

## 🎯 **Progressive Trust Building**

### **Phase 1: Conservative (Default)**
```yaml
balancing:
  aggressiveness: "low"      # 4h cooldown - very conservative
```
- ✅ Won't move VMs unnecessarily
- ✅ 4-hour cooldown per VM
- ✅ Earns trust through stability

### **Phase 2: After Trust Earned**
```yaml
balancing:
  aggressiveness: "medium"   # 2h cooldown - more responsive
```
- ✅ More responsive balancing
- ✅ 2-hour cooldown per VM
- ✅ Still conservative enough

### **Phase 3: High Performance**
```yaml
balancing:
  aggressiveness: "high"     # 30m cooldown - maximum optimization
```
- ✅ Maximum resource optimization
- ✅ 30-minute cooldown per VM
- ✅ Use only after proving reliability

## 🔧 **Configuration Philosophy**

- **Start Simple:** Zero configuration with sensible defaults
- **Earn Trust:** Conservative behavior by default (4h cooldown)
- **Progressive Tuning:** Increase aggressiveness after trust is earned
- **Advanced Features:** Enabled by default, but conservative in behavior
- **Always Enabled:** No "enabled" option - if you run it, you want balancing
- **Auto-Detection:** Cluster name auto-detected from Proxmox API
- **HTTPS by Default:** Proxmox always uses HTTPS, insecure=true for localhost
- **Zero Configuration:** Works without any config file in most cases

## 🎯 **Auto-Detection Features**

### **Cluster Name Auto-Detection**
```yaml
# No need to specify cluster name - auto-detected from Proxmox API
cluster:
  name: ""  # Auto-detected as "pve" (or your cluster name)
```

### **Proxmox HTTPS Simplification**
```yaml
# For localhost - no need to specify anything!
proxmox:
  host: "https://localhost:8006"  # Default
  insecure: true                  # Default for localhost (allows self-signed certs)
```

### **Remote Proxmox (credentials required)**
```yaml
proxmox:
  host: "https://192.168.1.100:8006"
  username: "root@pam"
  password: "your-password"
  insecure: false  # Use proper SSL for remote
```

## 🎯 **Cooldown Clarification**

**Cooldown is per-VM, not global:**
- ✅ VM-100 moved 2 hours ago → won't move VM-100 again for 2 more hours
- ✅ VM-101 never moved → can be moved immediately if needed
- ✅ Other VMs can still be balanced while VM-100 is in cooldown
- ✅ Not blocking balancing globally, just protecting recently moved VMs

## 🏆 **The MLP Result**

**What makes this lovable:**
- ✅ **Zero configuration** - Works without any config file!
- ✅ **Auto-detection** - Everything detected automatically
- ✅ **Conservative defaults** - Won't cause problems
- ✅ **Progressive complexity** - Start simple, tune later
- ✅ **Advanced features** - All features enabled by default

**Real-world usage:**
```bash
# Start with zero configuration:
goproxlb

# See it auto-detect:
# No config file specified, using defaults with auto-detection...
# Auto-detected cluster name: pve
# Starting GoProxLB...
# Using default configuration with auto-detection
# Proxmox host: https://localhost:8006
# Cluster: pve
# Balancing enabled: true
# Balancer type: advanced
# Aggressiveness: low
# Balancing interval: 5m0s
```

This approach ensures GoProxLB is **truly lovable from day one** with zero configuration while earning trust through reliable, conservative operation.
