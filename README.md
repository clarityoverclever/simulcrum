
---
# Simulcrum

## Overview
simulcrum provides deterministic DNS behavior for analysis, testing, and controlled environments. It replaces upstream DNS responses with either a static IP or a generated address from a spoofed subnet. The design emphasizes simplicity, transparency, and minimal overhead.

---

## Features
- Configurable DNS response address
- Optional upstream DNS liveness checks
- DNS spoofing using a configurable CIDR subnet
- Serves HTTP on configurable port
- Structured logging for debugging and monitoring
- Lightweight and easy to deploy
- Written in Go

---

## How It Works
1. simulcrum listens on a specified IP and port for DNS queries.
2. Each query is intercepted and, depending on configuration, rewritten with:
    - A static `analysis_ip`
    - The IP of the upstream DNS server with local DNAT
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
  Address and port simulcrum binds to for snooping DNS traffic.

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
  Address and port simulcrum binds to for serving HTTP traffic.
- 
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
```

### Usage
1. Edit the configuration file with your desired settings.
2. Ensure the listening port (typically 53) is available.
3. Start simulcrum (root/sudo recommended for privileged ports).
4. Query the DNS server and observe rewritten responses.

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