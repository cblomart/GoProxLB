# Build and CI/CD Guide

## Binary Builds

### Manual Builds

#### Using the Build Script (Recommended)
```bash
# Build with default version
./scripts/build.sh

# Build with specific version
VERSION=1.0.0 ./scripts/build.sh
```

#### Manual Commands
```bash
# Create build directory
mkdir -p build

# Build for Linux AMD64
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o build/goproxlb-linux-amd64 cmd/main.go

# Build for Linux ARM
GOOS=linux GOARCH=arm go build -ldflags="-s -w" -o build/goproxlb-linux-arm cmd/main.go

# Build for Linux ARM64
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o build/goproxlb-linux-arm64 cmd/main.go

# Strip binaries (optional, reduces size by ~30%)
strip build/goproxlb-linux-*

# Compress with UPX (optional, reduces size by ~70-74%)
upx --best --lzma build/goproxlb-linux-*

# Generate checksums
cd build
sha256sum goproxlb-linux-amd64 > goproxlb-linux-amd64.sha256
sha256sum goproxlb-linux-arm > goproxlb-linux-arm.sha256
sha256sum goproxlb-linux-arm64 > goproxlb-linux-arm64.sha256
```

### Build Flags Explained

- `-ldflags="-s -w"`: Strips debug information and reduces binary size
- `-X main.Version=${VERSION}`: Embeds version information
- `-X main.BuildTime=${BUILD_TIME}`: Embeds build timestamp

### UPX Compression

UPX (Ultimate Packer for eXecutables) provides excellent binary compression:

#### Installation
```bash
# macOS
brew install upx

# Ubuntu/Debian
sudo apt install upx

# CentOS/RHEL
sudo yum install upx

# Manual installation
wget https://github.com/upx/upx/releases/download/v4.2.1/upx-4.2.1-amd64_linux.tar.xz
tar -xf upx-4.2.1-amd64_linux.tar.xz
sudo mv upx-4.2.1-amd64_linux/upx /usr/local/bin/
```

#### Usage
```bash
# Best compression (recommended)
upx --best --lzma binary

# Fast compression
upx --fast binary

# Check if binary is compressed
upx -l binary
```

## GitHub Actions CI/CD

### Workflows

#### 1. CI Pipeline (`.github/workflows/ci.yml`)

**Triggers**: Push to main/develop, Pull Requests

**Jobs**:
- **Test**: Runs tests on multiple Go versions (1.20, 1.21, 1.22)
- **Lint**: Runs golangci-lint with comprehensive checks
- **Security**: Runs gosec and govulncheck for security scanning
- **Code Quality**: Checks cyclomatic complexity, static analysis
- **Build**: Builds binaries for all platforms
- **Docker Build**: Builds and caches Docker image
- **Integration Test**: Tests CLI functionality
- **Dependency Check**: Checks for outdated dependencies
- **License Check**: Validates license headers

#### 2. Release Pipeline (`.github/workflows/release.yml`)

**Triggers**: Push tags starting with 'v' (e.g., v1.0.0)

**Jobs**:
- **Release**: Builds static binaries, creates checksums, publishes to GitHub Releases
- **Docker**: Builds and pushes multi-platform Docker images to GHCR
- **Security Scan**: Runs Trivy vulnerability scanner on Docker images
- **Notify**: Provides release status notifications

### Features

#### Security Scanning
- **gosec**: Static security analysis
- **govulncheck**: Vulnerability scanning
- **Trivy**: Container vulnerability scanning
- **SARIF**: Security results integration with GitHub

#### Code Quality
- **golangci-lint**: Comprehensive linting
- **staticcheck**: Advanced static analysis
- **gocyclo**: Cyclomatic complexity checking
- **errcheck**: Error handling validation

#### Multi-Platform Support
- **Linux AMD64**: Standard x86_64 servers
- **Linux ARM**: ARM32 devices (Raspberry Pi, etc.)
- **Linux ARM64**: ARM64 servers (AWS Graviton, etc.)

## Docker Images

### Building Locally
```bash
# Build for current platform
docker build -t goproxlb .

# Build for specific platform
docker buildx build --platform linux/amd64 -t goproxlb:amd64 .

# Build multi-platform
docker buildx build --platform linux/amd64,linux/arm,linux/arm64 -t goproxlb .
```

### Using GitHub Container Registry
```bash
# Pull latest
docker pull ghcr.io/cblomart/goproxlb:latest

# Pull specific version
docker pull ghcr.io/cblomart/goproxlb:v1.0.0

# Run container
docker run -it --rm -v $(pwd)/config.yaml:/app/config.yaml ghcr.io/cblomart/goproxlb:latest
```

## Release Process

### Creating a Release

1. **Update Version**
   ```bash
   # Update version in code if needed
   # Commit changes
   git add .
   git commit -m "Prepare for v1.0.0"
   ```

2. **Create Tag**
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

3. **Automatic Release**
   - GitHub Actions automatically triggers
   - Builds binaries for all platforms
   - Creates GitHub Release with assets
   - Publishes Docker images to GHCR

### Release Assets

Each release includes:
- `goproxlb-{version}-linux-amd64.tar.gz`
- `goproxlb-{version}-linux-arm.tar.gz`
- `goproxlb-{version}-linux-arm64.tar.gz`
- `checksums.txt` (SHA256 checksums)
- Docker images: `ghcr.io/cedric/goproxlb:{version}`

## Configuration

### golangci-lint (`.golangci.yml`)

Comprehensive linting configuration including:
- Code formatting (gofmt, goimports)
- Static analysis (staticcheck, gosimple)
- Security scanning (gosec)
- Complexity checking (gocyclo)
- Error handling (errcheck)

### GitHub Actions Secrets

No additional secrets required for basic functionality. Uses:
- `GITHUB_TOKEN`: Automatically provided by GitHub
- Repository permissions: Configured in workflow files

## Monitoring and Notifications

### CI/CD Status
- All workflows run on every push/PR
- Status badges available for README
- Detailed logs in GitHub Actions tab

### Security Alerts
- Security scanning results in GitHub Security tab
- SARIF integration for vulnerability tracking
- Automated alerts for new vulnerabilities

## Troubleshooting

### Common Issues

1. **Build Failures**
   - Check Go version compatibility
   - Verify all dependencies are available
   - Check for syntax errors

2. **Lint Failures**
   - Run `golangci-lint run` locally
   - Check `.golangci.yml` configuration
   - Fix code style issues

3. **Security Failures**
   - Review gosec and govulncheck output
   - Update dependencies if needed
   - Address security vulnerabilities

4. **Docker Build Issues**
   - Check Dockerfile syntax
   - Verify build context
   - Check for missing files

### Local Development

```bash
# Run tests
go test ./...

# Run linting
golangci-lint run

# Run security scan
gosec ./...

# Build locally
./scripts/build.sh

# Test Docker build
docker build -t goproxlb .
```

## Performance

### Binary Sizes

#### Before UPX Compression
- **Linux AMD64**: ~9.2MB
- **Linux ARM**: ~8.9MB  
- **Linux ARM64**: ~8.7MB

#### After UPX Compression
- **Linux AMD64**: ~2.7MB (70% reduction)
- **Linux ARM**: ~2.3MB (74% reduction)
- **Linux ARM64**: ~2.4MB (72% reduction)

**Total size reduction: ~70-74%**

### Build Times
- **Local build**: ~30 seconds
- **CI build**: ~2-3 minutes
- **Docker build**: ~1-2 minutes

### Optimization
- **UPX Compression**: Reduces binary size by ~70-74%
- **Stripped Binaries**: Reduces size by ~30% (before UPX)
- **Static Linking**: Ensures portability
- **Multi-stage Docker Builds**: Minimize image size
- **LZMA Compression**: Best compression ratio with UPX
