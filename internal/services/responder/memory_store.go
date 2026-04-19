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

package responder

import (
	"context"
	"sync"
	"time"
)

type MemoryStore struct {
	records map[Key]Record
	mutex   sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		records: make(map[Key]Record),
	}
}

// cloneRecord returns a deep copy of the given record for a vm
func cloneRecord(record Record) Record {
	clone := record

	if record.Tags != nil {
		clone.Tags = make(map[string]struct{}, len(record.Tags))
		for k, v := range record.Tags {
			clone.Tags[k] = v
		}
	}

	if record.Meta != nil {
		clone.Meta = make(map[string]string, len(record.Meta))
		for k, v := range record.Meta {
			clone.Meta[k] = v
		}
	}

	return clone
}

func (s *MemoryStore) Observe(ctx context.Context, key Key, obs Observation) (Record, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	record, ok := s.records[key]
	if !ok {
		record = Record{
			Key:       key,
			Tags:      make(map[string]struct{}),
			Meta:      make(map[string]string),
			FirstSeen: obs.Timestamp,
		}
	}

	if record.Tags == nil {
		record.Tags = make(map[string]struct{})
	}

	if record.Meta == nil {
		record.Meta = make(map[string]string)
	}

	record.LastSeen = obs.Timestamp

	switch obs.Kind {
	case KindDNS:
		record.DNSQueries++
	case KindHTTP:
		record.HTTPRequests++
	case KindTLS:
		record.TLSHandshakes++
	}

	for k, v := range obs.Meta {
		record.Meta[k] = v
	}

	s.records[key] = record

	return cloneRecord(record), nil
}

func (s *MemoryStore) Get(ctx context.Context, key Key) (Record, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	record, ok := s.records[key]
	return cloneRecord(record), ok
}

func (s *MemoryStore) GetOrCreate(ctx context.Context, key Key) (Record, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	record, ok := s.records[key]
	if !ok {
		record = Record{
			Key:  key,
			Tags: make(map[string]struct{}),
			Meta: make(map[string]string),
		}
		s.records[key] = record
	}

	return record, nil
}

func (s *MemoryStore) UpdateMeta(ctx context.Context, key Key, values map[string]string) (Record, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	record, ok := s.records[key]
	if !ok {
		return Record{}, nil
	}

	if record.Meta == nil {
		record.Meta = make(map[string]string)
	}

	for k, v := range values {
		record.Meta[k] = v
	}

	s.records[key] = record
	return cloneRecord(record), nil
}

func (s *MemoryStore) AddTag(ctx context.Context, key Key, tag string) (Record, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	record, ok := s.records[key]
	if !ok {
		return Record{}, nil
	}

	if record.Tags == nil {
		record.Tags = make(map[string]struct{})
	}

	record.Tags[tag] = struct{}{}
	s.records[key] = record
	return cloneRecord(record), nil
}

func (s *MemoryStore) RemoveTag(ctx context.Context, key Key, tag string) (Record, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	record, ok := s.records[key]
	if !ok {
		return Record{}, nil
	}

	if record.Tags != nil {
		delete(record.Tags, tag)
	}

	s.records[key] = record
	return cloneRecord(record), nil
}

func (s *MemoryStore) MarkSeen(ctx context.Context, key Key, at time.Time) (Record, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	record, ok := s.records[key]
	if !ok {
		return Record{}, nil
	}

	if record.FirstSeen.IsZero() {
		record.FirstSeen = at
	}
	record.LastSeen = at

	s.records[key] = record
	return cloneRecord(record), nil
}

func (s *MemoryStore) Delete(ctx context.Context, key Key) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.records, key)
	return nil
}

func (s *MemoryStore) PruneExpired(ctx context.Context, olderThan time.Time) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	count := 0
	for key, record := range s.records {
		if record.LastSeen.Before(olderThan) {
			delete(s.records, key)
			count++
		}
	}

	return count, nil
}
