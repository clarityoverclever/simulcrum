
---
# simulacrum

## Overview
Simulacrum aims to provide deterministic network behavior for analysis and testing in controlled environments.

---

## Features
- supplies configurable servers on a data plane (DNS, HTTP(S), NTP)
- exposes servers to a control plane for dynamic configuration
- structured logging for analysis
- real-time reporting of server behavior

### DNS
- Serves DNS on configurable port
- Rewrites queries to a static IP or upstream DNS server with local DNAT/
- Optional "liveness" checks against upstream DNS server
- DNS spoofing using a configurable CIDR subnet

### HTTP(S)
- Serves HTTP with file service on configurable port
- Capture POST data into Base64 files for later analysis
- Optional logging of HTTP request headers
- Optional spoofed HTTP request payload delivery (ps1, exe)

### TLS
- Manages TLS certificates for HTTPS

### NTP
- Serves NTP
- Supports adding a time multiplier to NTP datagram

---

## How to Use
```bash
git clone https://github.com/simulacrum/simulacrum.git

cd simulacrum

# build data plane
go build ./cmd/simulacrum/simulacrum.go

# build control plane
go build ./cmd/simctl/simctl.go
````

## How It Works
1. simulacrum listens on a specified IP and port for DNS queries.
2. Each query is intercepted and, depending on configuration, rewritten with:
    - A static `analysis_ip`
    - The IP of the upstream DNS server with local DNAT redirection
    - A generated IP from `default_subnet` when spoofing is enabled.
3. Optional liveness checks validate upstream DNS availability.
4. Logs provide visibility into query flow and behavior.

---

## Configuration
file: ./config/config.yaml

### dns
- **enabled:** `true | false`  
  Controls whether the DNS server starts at launch.

- **bind_addr:** `IP:PORT`  
  Address and port simulacrum binds to for snooping DNS traffic.

- **analysis_ip:** `IP`  
  The IP returned for all DNS queries when spoofing is disabled.

- **check_liveness:** `true | false`  
  Enables upstream DNS health checks.

- **upstream_dns:** `IP:PORT`  
  Required when `check_liveness` is enabled.

- **spoof_network:** `true | false`  
  Enables spoofed DNS responses.

- **default_subnet:** `CIDR`  
  Subnet used to generate spoofed IPs.

### http
- **enabled:** `true | false`  
  Controls whether the HTTP server starts at launch.

- **bind_addr:** `IP:PORT`  
  Address and port simulacrum binds to for serving HTTP traffic.

### https
- **enabled:** `true | false`  
  Controls whether the HTTP server starts at launch.

- **bind_addr:** `IP:PORT`  
  Address and port simulacrum binds to for serving HTTP traffic.

### common_web:
- **max_body_kbe:** `int`  
  Maximum capture size of HTTP POST request bodies in kilobytes.

- **log_headers:** `true | false`  
  Enables logging of HTTP request headers.

- **spoof_payload:** `true | false`  
  Enables spoofing of HTTP request payloads (ps1, exe, binary.

### tls
- **cert_mode:** `static`
  Controls how TLS certificates are managed.

- **cert_file:** `PATH`
  Path to TLS certificate file.

- **key_file:** `PATH`
  Path to TLS key file.

### ntp
- **enabled:** `true | false`  
  Controls whether the NTP server starts at launch.

- **bind_addr:** `IP:PORT`  
  Address and port serving NTP.

- **multiplier:** `float`  
  Multiplier applied to NTP timestamps.

### Example
```yaml
dns:
  enabled: true
  bind_addr: 0.0.0.0:53
  analysis_ip: 192.168.117.128
  check_liveness: true
  upstream_dns: 9.9.9.9:53
  spoof_network: true
  default_subnet: 10.0.1.0/8
http:
  enabled: true
  bind_addr: 0.0.0.0:80
https:
  enabled: true
  bind_addr: 0.0.0.0:443
common_web:
  log_headers: true
  spoof_payload: true
  max_body_kb: 64
tls:
  cert_mode: static
  cert_file: ./certs/https.crt
  key_file: ./certs/https.key
ntp:
  enabled: true
  bind_addr: 0.0.0.0:123
  multiplier: 1.0
```

### Usage
1. Edit the configuration file with your desired settings.
2. Ensure the listening port (typically 53) is available.
3. Start simulacrum (root/sudo recommended for privileged ports).

### Notes
#### Enable IP forwarding on host for spoofing
```bash
sudo sysctl -w net.ipv4.ip_forward=1

# add persistent IP forwarding if needed
echo "net.ipv4.ip_forward=1" | sudo tee -a /etc/sysctl.conf
```

#### Inspect PREROUTING NAT table
```bash
sudo iptables -t nat -L PREROUTING -n -v
```

#### Build exe agent
```bash
GOOS=windows GOARCH=amd64 go build -o agent.exe ./cmd/agent/agent.go

mv ./agent.exe ./internal/services/http/static/
```