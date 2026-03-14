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

package tlscert

import (
	"crypto/tls"
	"fmt"
)

type CertificateProvider interface {
	GetCertificate(serverName string) (*tls.Certificate, error)
}

type Issuer interface {
	IssueServerCertificate(serverName string) (*tls.Certificate, error)
}

type StaticProvider struct {
	Certificate *tls.Certificate
}

func (p *StaticProvider) GetCertificate(serverName string) (*tls.Certificate, error) {
	if p == nil || p.Certificate == nil {
		return nil, fmt.Errorf("no certificate provided")
	}

	return p.Certificate, nil
}
