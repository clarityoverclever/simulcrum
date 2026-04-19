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

package dns

import (
	"context"
	"fmt"
	"net"
	"os"
	"simulacrum/internal/core/hash"
	"simulacrum/internal/core/inspect"
	"simulacrum/internal/core/logger"
	"simulacrum/internal/services/dns/dnat"
	"simulacrum/internal/services/dns/ippool"
	"simulacrum/internal/services/responder"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/net/publicsuffix"
)

type Server struct {
	bindAddress              string
	analysisIP               net.IP
	dnsUdpServer             *dns.Server
	dnsTcpServer             *dns.Server
	VerifyUpstream           bool
	upstreamDNS              string
	tunnelDetectionThreshold float64
	responderManager         *responder.Manager
	ipPool                   *ippool.Pool
	ignoreList               *ignore
	dnatManager              *dnat.Manager
	dnatMap                  map[string]string // domain -> spoofed IP
	dnatLock                 sync.RWMutex
}

type Config struct {
	Enabled                  bool
	BindAddress              string
	AnalysisIP               string
	VerifyUpstream           bool
	UpstreamDNS              string
	DefaultSubnet            string
	TunnelDetectionThreshold float64
	ResponseManager          *responder.Manager
}

func New(cfg Config) (*Server, error) {
	var err error

	ip := net.ParseIP(cfg.AnalysisIP)
	if ip == nil {
		return nil, fmt.Errorf("invalid analysis IP address: %s", cfg.AnalysisIP)
	}

	var pool *ippool.Pool
	pool, err = ippool.New(cfg.DefaultSubnet)
	if err != nil {
		return nil, fmt.Errorf("invalid default subnet: %w", err)
	}

	ignoreList := newIgnoreList()

	server := &Server{
		bindAddress:              cfg.BindAddress,
		analysisIP:               ip,
		VerifyUpstream:           cfg.VerifyUpstream,
		upstreamDNS:              cfg.UpstreamDNS,
		tunnelDetectionThreshold: cfg.TunnelDetectionThreshold,
		responderManager:         cfg.ResponseManager,
		ipPool:                   pool,
		ignoreList:               ignoreList,
		dnatManager:              dnat.New(cfg.AnalysisIP),
		dnatMap:                  make(map[string]string),
	}

	// validate upstream DNS if liveness check is enabled
	if server.VerifyUpstream && server.upstreamDNS == "" {
		return nil, fmt.Errorf("upstream DNS required for liveness check")
	}

	return server, nil
}

// Start starts the DNS server.
func (s *Server) Start() error {
	dns.HandleFunc(".", s.handleDNSRequest)

	s.dnsUdpServer = &dns.Server{Addr: s.bindAddress, Net: "udp"}
	s.dnsTcpServer = &dns.Server{Addr: s.bindAddress, Net: "tcp"}

	errChan := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := s.dnsUdpServer.ListenAndServe(); err != nil {
			errChan <- fmt.Errorf("failed to open UDP listener: %w", err)
		} else {
			errChan <- nil
		}
	}()
	fmt.Println("[dns] listening on UDP", s.bindAddress)

	go func() {
		defer wg.Done()
		if err := s.dnsTcpServer.ListenAndServe(); err != nil {
			errChan <- fmt.Errorf("failed to open TCP listener: %w", err)
		} else {
			errChan <- nil
		}
	}()
	fmt.Println("[dns] listening on TCP", s.bindAddress)

	err := <-errChan

	_ = s.dnsUdpServer.Shutdown()
	_ = s.dnsTcpServer.Shutdown()

	wg.Wait()

	return err
}

// Stop stops the DNS server.
func (s *Server) Stop() error {
	if s.dnsUdpServer != nil || s.dnsTcpServer != nil {
		fmt.Println("[dns] stopping server")

		// Clean up all DNAT rules
		s.dnatLock.Lock()
		fmt.Println("[dns] removing DNAT rules")

		for domain, spoofedIP := range s.dnatMap {
			if err := s.dnatManager.RemoveDNAT(spoofedIP); err != nil {
				logger.Error("[dns] failed to remove DNAT", "domain", domain, "error", err)
			}
		}
		s.dnatLock.Unlock()

		// Clear IP pool
		if s.ipPool != nil {
			s.ipPool.Clear()
		}

		var err error
		err = s.dnsUdpServer.Shutdown()
		if err != nil {
			return fmt.Errorf("failed to shutdown dns UDP server: %w", err)
		}
		err = s.dnsTcpServer.Shutdown()
		if err != nil {
			return fmt.Errorf("failed to shutdown dns TCP server: %w", err)
		}

		return nil
	}
	return nil
}

// verifyUpstreamDNS checks the live status of a domain by querying an upstream DNS server.
func (s *Server) verifyUpstreamDNS(ctx context.Context, domain string, qtype uint16, isSuspectedTunnel bool) (bool, net.IP) {
	if !s.VerifyUpstream {
		return true, nil // always return success if upstream checking is disabled
	}

	var err error
	target := domain

	if isSuspectedTunnel {
		// extract apex domain for the upstream query to counter dns tunneling
		domain = strings.TrimSpace(strings.TrimSuffix(domain, "."))
		target, err = publicsuffix.EffectiveTLDPlusOne(domain)
		if err != nil {
			logger.WarnContext(ctx, "[dns] failed to resolve apex domain", "domain", domain, "error", err)
			return true, nil // fail open if apex domain isolation fails
		}
	}

	c := new(dns.Client)
	c.Timeout = 2 * time.Second

	visited := make(map[string]struct{})

	return s.resolveUpstreamIP(ctx, c, target, qtype, 0, visited)
}

// resolveUpstreamIP resolves an IP address for a domain by querying an upstream DNS server.
func (s *Server) resolveUpstreamIP(ctx context.Context, c *dns.Client, domain string, qtype uint16, depth int, visited map[string]struct{}) (bool, net.IP) {
	const maxCNAMEHops = 5

	domain = strings.TrimSpace(strings.TrimSuffix(domain, "."))
	if domain == "" {
		return true, nil
	}

	if _, seen := visited[domain]; seen {
		logger.WarnContext(ctx, "[dns] CNAME loop detected",
			"domain", domain,
			"type", dns.TypeToString[qtype],
		)
		return true, nil
	}
	visited[domain] = struct{}{}

	if depth > maxCNAMEHops {
		logger.WarnContext(ctx, "[dns] max CNAME hop limit reached",
			"domain", domain,
			"type", dns.TypeToString[qtype],
		)
		return true, nil
	}

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), qtype)
	m.RecursionDesired = true

	r, _, err := c.Exchange(m, s.upstreamDNS)
	if err != nil {
		logger.WarnContext(ctx, "[dns] upstream DNS check failed",
			"domain", domain,
			"error", err,
			"type", dns.TypeToString[qtype])
		return true, nil // fail open if upstream check fails
	}

	// check response code
	switch r.Rcode {
	case dns.RcodeSuccess:
		if qtype != dns.TypeA {
			logger.InfoContext(ctx, "[dns] upstream DNS check succeeded",
				"domain", domain,
				"type", dns.TypeToString[qtype],
			)
			return true, nil
		}

		// look for a direct A record.
		for _, ans := range r.Answer {
			if a, ok := ans.(*dns.A); ok && a.A != nil {
				logger.InfoContext(ctx, "[dns] upstream DNS check succeeded",
					"domain", domain,
					"type", dns.TypeToString[qtype],
					"upstream_ip", a.A,
				)
				return true, a.A
			}
		}

		// follow CNAME chain if A record is not found
		// recursion depth 5
		for _, ans := range r.Answer {
			if cname, ok := ans.(*dns.CNAME); ok && cname.Target != "" {
				logger.InfoContext(ctx, "[dns] following CNAME chain", "domain", domain, "target", cname.Target, "type", dns.TypeToString[qtype])
				nextDomain := strings.TrimSpace(strings.TrimSuffix(cname.Target, "."))
				return s.resolveUpstreamIP(ctx, c, nextDomain, dns.TypeA, depth+1, visited)
			}
		}

		logger.InfoContext(ctx, "[dns] upstream DNS check succeeded but no A record was returned",
			"domain", domain,
			"type", dns.TypeToString[qtype],
		)
		return true, nil

	case dns.RcodeNameError: // NXDOMAIN
		logger.InfoContext(ctx, "[dns] upstream DNS check failed",
			"domain", domain,
			"error", "NXDOMAIN",
		)
		return false, nil

	default:
		logger.WarnContext(ctx, "[dns] upstream DNS check failed",
			"domain", domain,
			"rcode", dns.RcodeToString[r.Rcode],
		)
		return true, nil // fail open
	}
}

// testIsAliveUpstream checks the live status of a domain with an SOA query.
func (s *Server) testIsAliveUpstream(ctx context.Context, domain string) bool {
	if !s.VerifyUpstream {
		return true // always return success if upstream checking is disabled
	}

	domain = strings.TrimSpace(strings.TrimSuffix(domain, "."))
	apex, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		logger.WarnContext(ctx, "[dns] failed to resolve apex domain", "domain", domain, "error", err)
		return true // fail open if apex domain isolation fails
	}

	c := new(dns.Client)
	c.Timeout = 2 * time.Second

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(apex), dns.TypeSOA)
	m.RecursionDesired = true

	r, _, err := c.Exchange(m, s.upstreamDNS)
	if err != nil {
		logger.WarnContext(ctx, "[dns] upstream DNS check failed",
			"domain", domain,
			"error", err)
		return true // fail open if upstream check fails
	}

	// check response code
	switch r.Rcode {
	case dns.RcodeSuccess:
		logger.InfoContext(ctx, "[dns] upstream DNS check succeeded",
			"domain", domain,
		)
		return true
	case dns.RcodeNameError: // NXDOMAIN
		logger.InfoContext(ctx, "[dns] upstream DNS check failed",
			"domain", domain,
			"error", "NXDOMAIN",
		)
		return false
	default:
		logger.WarnContext(ctx, "[dns] upstream DNS check failed",
			"domain", domain,
			"rcode", dns.RcodeToString[r.Rcode],
		)
		return true // fail open
	}
}

// testEntropy checks the entropy of a domain to detect potential tunneling.
func (s *Server) testEntropy(target string) (bool, float64) {
	labels := strings.Split(strings.TrimSuffix(target, "."), ".")
	suspicious := labels[0]

	entropy := inspect.Shannon([]byte(suspicious))

	if entropy > s.tunnelDetectionThreshold {
		return true, entropy
	}
	return false, entropy
}

// resolveProvisioning returns an IP address for a domain based on provisioning mode.
func (s *Server) resolveProvisioning(ctx context.Context, mode responder.Provisioning, upstreamIP net.IP, domain string) (net.IP, error) {
	switch mode {
	case "static": // allocate an IP from the default subnet
		ip, err := s.ipPool.Allocate()
		if err != nil {
			return nil, fmt.Errorf("failed to allocate static IP: %w", err)
		}

		if err = s.addDNAT(ip, domain); err != nil {
			return nil, err
		}

		return ip, nil
	case "dynamic": // use the upstream IP and fall back to static if unavailable
		if upstreamIP != nil {
			if err := s.addDNAT(upstreamIP, domain); err != nil {
				return nil, err
			}
			return upstreamIP, nil
		}

		logger.WarnContext(ctx, "[dns] upstream IP not available for dynamic provisioning, falling back to static", "domain", domain)

		ip, err := s.ipPool.Allocate()
		if err != nil {
			return nil, fmt.Errorf("failed to allocate fallback IP: %w", err)
		}

		if err = s.addDNAT(ip, domain); err != nil {
			return nil, err
		}

		return ip, nil
	case "none": // do not provision an IP
		return nil, nil
	default:
		logger.WarnContext(ctx, "[dns] unknown provisioning mode", "mode", mode)
		return s.analysisIP, nil
	}
}

// addDNAT adds a DNAT rule for the given IP and domain.
func (s *Server) addDNAT(IP net.IP, domain string) error {
	if IP == nil {
		return fmt.Errorf("cannot add DNAT for nil IP")
	}

	// Check if the domain is already mapped
	s.dnatLock.Lock()
	if _, ok := s.dnatMap[domain]; ok {
		s.dnatLock.Unlock()
		return nil
	}
	s.dnatLock.Unlock()

	if err := s.dnatManager.AddDNAT(IP.String()); err != nil {
		return fmt.Errorf("failed to add DNAT: %w", err)
	}

	// Track mapping for cleanup
	s.dnatLock.Lock()
	s.dnatMap[domain] = IP.String()
	s.dnatLock.Unlock()

	return nil
}

func (s *Server) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	var err error
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true
	msg.Rcode = dns.RcodeSuccess

	clientAddr := w.RemoteAddr().String()
	clientIP, _, _ := net.SplitHostPort(clientAddr)

	// set default response IP to analysis IP
	var responseIP net.IP
	responseIP = s.analysisIP

	for _, question := range r.Question {
		reqID := hash.GetReqID()
		queryCtx := logger.WithReqID(context.Background(), reqID)

		// extract domain with subdomain from question
		domain := strings.TrimSuffix(question.Name, ".")

		// check if the domain has been ignored and set default values for domain metadata
		ignored := s.ignoreList.checkIsIgnored(domain)
		isSuspectedTunnel := false
		score := 0.0
		testedUpstream := false
		isResolvedUpstream := true
		upstreamIP := net.IP{}

		// collect data for non-ignored domains
		if !ignored {
			logger.InfoContext(queryCtx, "[dns] query",
				"domain", domain,
				"type", dns.TypeToString[question.Qtype],
				"client", clientAddr,
			)

			// start metadata collection for response manager

			// attempt to detect tunneling by testing domain entropy
			isSuspectedTunnel, score = s.testEntropy(question.Name)

			if isSuspectedTunnel {
				logger.WarnContext(queryCtx, "[dns] possible tunneling detected", "domain", domain, "entropy", score)
			}

			// check upstream DNS for domain if liveness check is enabled
			testedUpstream = s.VerifyUpstream
			isResolvedUpstream, upstreamIP = s.verifyUpstreamDNS(queryCtx, question.Name, question.Qtype, isSuspectedTunnel)

			// return NXDOMAIN if upstream check fails
			if !isResolvedUpstream {
				logger.InfoContext(queryCtx, "[dns] returning NXDOMAIN for non-existent domain", "domain", domain)

				msg.SetRcode(r, dns.RcodeNameError)
				err = s.writeDNSResponse(w, msg)

				continue
			}
		}

		// send data to response manager

		if s.responderManager != nil {
			req := responder.RequestContext{
				Key:    responder.Key(domain),
				Kind:   responder.KindDNS,
				Source: clientIP,
				Target: domain,
				Inputs: responder.Inputs{
					IsSuspectedTunnel: isSuspectedTunnel,
					Entropy:           score,
					TestedUpstream:    testedUpstream,
					IsAlive:           isResolvedUpstream,
				},
				Meta: map[string]string{"qtype": dns.TypeToString[question.Qtype]},
				Now:  time.Now(),
			}

			var result responder.Result
			if result, err = s.responderManager.Handle(queryCtx, req); err != nil {
				logger.WarnContext(queryCtx, "[dns] responder handle error", "error", err)
			}

			// process resolver results

			// process internal resolver errors
			if result.Mode == "error" {
				// todo implement lifecycle management for vm in error state
				err = s.actionHandler(queryCtx, result.Actions)
				if err != nil {
					logger.WarnContext(queryCtx, "[dns] action handler error", "error", err)
				}

				msg.SetRcode(r, dns.RcodeServerFailure)
				err = s.writeDNSResponse(w, msg)
				if err != nil {
					logger.WarnContext(queryCtx, "[dns] failed to write response after error mode", "error", err)
				}
				continue
			}

			// process ignore mode before provisioning
			if result.Mode == "ignore" {
				err = s.actionHandler(queryCtx, result.Actions)
				if err != nil {
					logger.WarnContext(queryCtx, "[dns] action handler error", "error", err)
				}

				switch result.Response.ResponseCode {
				case "NXDOMAIN":
					msg.SetRcode(r, dns.RcodeNameError)
				case "SERVFAIL":
					msg.SetRcode(r, dns.RcodeServerFailure)
				case "REFUSED":
					msg.SetRcode(r, dns.RcodeRefused)
				default:
					msg.SetRcode(r, dns.RcodeSuccess)
				}

				// add domain to ignore list
				if !ignored {
					logger.InfoContext(queryCtx, "[dns] ignoring domain", "domain", domain)
					s.ignoreList.add(domain)
				}

				err = s.writeDNSResponse(w, msg)
				if err != nil {
					logger.WarnContext(queryCtx, "[dns] failed to write response after ignore mode", "error", err)
				}

				continue
			}

			switch result.Mode {
			case "spoof":
				// remove previously ignored domain
				if ignored {
					logger.InfoContext(queryCtx, "[dns] removing ignored domain", "domain", domain)
					s.ignoreList.remove(domain)
				}

				// provision an IP based on responder directive
				responseIP, err = s.resolveProvisioning(queryCtx, result.Response.Provisioning, upstreamIP, domain)
				if err != nil || responseIP == nil {
					logger.WarnContext(queryCtx, "[dns] provisioning resolution failed, fallback to analysisIP", "domain", domain, "error", err)
					// fallback to analysisIP if provisioning fails
					responseIP = s.analysisIP
				}

				msg = s.handleSpoofRequest(queryCtx, result.Response, responseIP, question, msg)

				logger.InfoContext(queryCtx, "[dns] response",
					"domain", domain,
					"returned_ip", responseIP.String(),
				)
			case "proxy":
				// remove previously ignored domain
				if ignored {
					logger.InfoContext(queryCtx, "[dns] removing ignored domain", "domain", domain)
					s.ignoreList.remove(domain)
				}

				msg, err = s.handleProxyRequest(queryCtx, w, r)
			default:
				logger.WarnContext(queryCtx, "[dns] unknown mode", "mode", result.Mode)
				continue
			}

			// handle actions
			err = s.actionHandler(queryCtx, result.Actions)
			if err != nil {
				logger.WarnContext(queryCtx, "[dns] action handler error", "error", err)
			}
		}

		err = s.writeDNSResponse(w, msg)
		if err != nil {
			logger.WarnContext(queryCtx, "[dns] failed to write response", "error", err)
		}
	}
}

// writeDNSResponse writes a supplied DNS response to the client
func (s *Server) writeDNSResponse(w dns.ResponseWriter, msg *dns.Msg) error {
	if msg == nil {
		msg = new(dns.Msg)
		msg.SetRcode(msg, dns.RcodeServerFailure)
	}

	if writeErr := w.WriteMsg(msg); writeErr != nil {
		_, writeErr = fmt.Fprintf(os.Stderr, "dns write failure: %v\n", writeErr)
		if writeErr != nil {
			return writeErr
		}
	}
	return nil
}
