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
	"net"
	"simulacrum/internal/core/logger"
	"simulacrum/internal/services/responder"
	"strings"

	"github.com/miekg/dns"
)

func (s *Server) handleSpoofRequest(ctx context.Context, r responder.Response, ip net.IP, question dns.Question, msg *dns.Msg) *dns.Msg {
	switch strings.ToUpper(string(r.RecordType)) {
	case "A":
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: header(question.Name, dns.TypeA, 1),
			A:   ip,
		})
		return msg
	case "AAAA":
		logger.InfoContext(ctx, "[dns] returning NODATA for AAAA query", "domain", question.Name)
		// sending NODATA for AAAA queries to attempt to force IPv4 fallback
	case "TXT":
		msg.Answer = append(msg.Answer, &dns.TXT{
			Hdr: header(question.Name, dns.TypeTXT, 1),
			Txt: []string{r.Value},
		})
		return msg
	case "MX", "NS", "CNAME":
		logger.InfoContext(ctx, "[dns] ignoring unsupported query", "type", dns.TypeToString[question.Qtype])
		// TODO add capture support for these types
		// sending NODATA for unsupported types
	default:
		logger.InfoContext(ctx, "[dns] unknown query type", "type", question.Qtype)
		msg.SetRcode(msg, dns.RcodeNameError)
	}

	return msg.SetRcode(msg, dns.RcodeServerFailure)
}

// header returns a DNS header with a supplied name, type, and TTL
func header(name string, rrtype uint16, ttl uint32) dns.RR_Header {
	return dns.RR_Header{
		Name:   name,
		Rrtype: rrtype,
		Class:  dns.ClassINET,
		Ttl:    ttl,
	}
}
