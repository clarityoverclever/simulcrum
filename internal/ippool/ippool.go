// Copyright 2026 Keith Marshall
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ippool

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
)

// Pool manages IP allocation from a subnet
type Pool struct {
	subnet       *net.IPNet
	allocatedIPs map[string]bool // Track used IPs
	mu           sync.RWMutex
}

// New creates a new IP pool from a CIDR subnet
func New(cidr string) (*Pool, error) {
	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}

	return &Pool{
		subnet:       subnet,
		allocatedIPs: make(map[string]bool),
	}, nil
}

// Allocate returns a random IP from the subnet
func (p *Pool) Allocate() (net.IP, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Get network range
	ip := p.subnet.IP
	mask := p.subnet.Mask

	// Calculate usable IPs (skip network and broadcast)
	ones, bits := mask.Size()
	hostBits := bits - ones
	maxHosts := (1 << hostBits) - 2 // Exclude network and broadcast

	if maxHosts <= 0 {
		return nil, fmt.Errorf("subnet too small")
	}

	// Try to find an unused IP (max 100 attempts)
	for attempt := 0; attempt < 100; attempt++ {
		// Generate random offset (skip 0 = network address)
		offset := rand.Intn(maxHosts) + 1

		// Calculate IP
		newIP := make(net.IP, len(ip))
		copy(newIP, ip)

		// Add offset to IP
		for i := len(newIP) - 1; i >= 0 && offset > 0; i-- {
			sum := int(newIP[i]) + offset
			newIP[i] = byte(sum & 0xff)
			offset = sum >> 8
		}

		// Check if IP is already allocated
		ipStr := newIP.String()
		if !p.allocatedIPs[ipStr] {
			p.allocatedIPs[ipStr] = true
			return newIP, nil
		}
	}

	return nil, fmt.Errorf("failed to allocate unique IP after 100 attempts")
}

// IsAllocated checks if an IP is already in use
func (p *Pool) IsAllocated(ip string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.allocatedIPs[ip]
}

// Release marks an IP as available (for cleanup)
func (p *Pool) Release(ip string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.allocatedIPs, ip)
}

// Count returns the number of allocated IPs
func (p *Pool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.allocatedIPs)
}

// Clear releases all IPs (for shutdown)
func (p *Pool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.allocatedIPs = make(map[string]bool)
}
