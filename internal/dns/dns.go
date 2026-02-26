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
	"fmt"
	"net"
	"os"
	"simulcrum/internal/logger"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type Server struct {
	addr          string
	defaultIP     net.IP
	dnsServer     *dns.Server
	upstreamDNS   string
	checkLiveness bool
}

type Config struct {
	ListenAddr    string
	DefaultIP     string // IP returned for all queries
	UpstreamDNS   string // DNS server to forward queries
	CheckLiveness bool   // use upstream DNS to test server liveness
}

func New(cfg Config) (*Server, error) {
	ip := net.ParseIP(cfg.DefaultIP)
	if ip == nil {
		return nil, fmt.Errorf("invalid default IP address: %s", cfg.DefaultIP)
	}
	server := &Server{
		addr:          cfg.ListenAddr,
		defaultIP:     ip,
		upstreamDNS:   cfg.UpstreamDNS,
		checkLiveness: cfg.CheckLiveness,
	}

	// validate upstream DNS if liveness check is enabled
	if server.checkLiveness && server.upstreamDNS == "" {
		return nil, fmt.Errorf("upstream DNS required for liveness check")
	}

	return server, nil
}

func (s *Server) Start() error {
	dns.HandleFunc(".", s.handleDNSRequest)

	s.dnsServer = &dns.Server{Addr: s.addr, Net: "udp"}

	fmt.Fprintf(os.Stdout, "Starting DNS server on %s\n", s.addr)

	if err := s.dnsServer.ListenAndServe(); err != nil {
		return fmt.Errorf("failed to start DNS server: %w", err)
	}
	return nil
}

func (s *Server) Stop() error {
	if s.dnsServer != nil {
		fmt.Println("stopping DNS server")
		return s.dnsServer.Shutdown()
	}
	return nil
}

func (s *Server) checkDomain(domain string, qtype uint16) bool {
	if !s.checkLiveness {
		return true // always return success if upstream checking is disabled
	}

	c := new(dns.Client)
	c.Timeout = 2 * time.Second

	m := new(dns.Msg)
	m.SetQuestion(domain, qtype)
	m.RecursionDesired = true

	r, _, err := c.Exchange(m, s.upstreamDNS)
	if err != nil {
		logger.Warn("upstream DNS check failed",
			"domain", domain,
			"error", err,
			"type", dns.TypeToString[qtype])
		return true // fail open if upstream check fails
	}

	// check response code
	switch r.Rcode {
	case dns.RcodeSuccess:
		logger.Info("upstream DNS check succeeded",
			"domain", domain,
			"type", dns.TypeToString[qtype],
		)
		return true
	case dns.RcodeNameError: // NXDOMAIN
		logger.Info("upstream DNS check failed",
			"domain", domain,
			"error", "NXDOMAIN",
		)
		return false
	default:
		logger.Warn("upstream DNS check failed",
			"domain", domain,
			"rcode", dns.RcodeToString[r.Rcode],
		)
		return true // fail open
	}
}

func (s *Server) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true

	for _, question := range r.Question {
		domain := strings.TrimSuffix(question.Name, ".")

		logger.Info("dns query",
			"domain", domain,
			"type", dns.TypeToString[question.Qtype],
			"client", w.RemoteAddr().String(),
		)

		// check upstream DNS for domain if liveness check is enabled
		if !s.checkDomain(question.Name, question.Qtype) {
			// return NXDOMAIN if upstream check fails
			msg.SetRcode(r, dns.RcodeNameError)
			logger.Info("returning NXDOMAIN for non-existent domain", "domain", domain)

			if err := w.WriteMsg(msg); err != nil {
				fmt.Fprintf(os.Stderr, "failed to write DNS response: %v\n", err)
			}

			return
		}

		switch question.Qtype {
		case dns.TypeA:
			msg.Answer = append(msg.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    30,
				},
				A: s.defaultIP,
			})
		case dns.TypeAAAA:
			logger.Info("ignoring AAAA query", "domain", question.Name)
		case dns.TypeMX, dns.TypeNS, dns.TypeCNAME, dns.TypeTXT:
			logger.Info("ignoring unsupported query", "type", dns.TypeToString[question.Qtype])
		default:
			logger.Info("unknown query type", "type", question.Qtype)
		}
	}

	if err := w.WriteMsg(msg); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write DNS response: %v\n", err)
	}
}
