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
	"sync"
)

type CachingProvider struct {
	issuer Issuer

	mu    sync.RWMutex
	cache map[string]*tls.Certificate
}

func NewCachingProvider(issuer Issuer) *CachingProvider {
	return &CachingProvider{
		issuer: issuer,
		cache:  make(map[string]*tls.Certificate),
	}
}

func (p *CachingProvider) GetCertificate(serverName string) (*tls.Certificate, error) {
	name, err := NormalizeServerName(serverName)
	if err != nil {
		return nil, err
	}

	p.mu.RLock()
	cert, ok := p.cache[name]
	p.mu.RUnlock()
	if ok {
		return cert, nil
	}

	if p.issuer == nil {
		return nil, fmt.Errorf("certificate issuer is not configured")
	}

	cert, err = p.issuer.IssueServerCertificate(name)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.cache[name] = cert
	p.mu.Unlock()

	return cert, nil
}
