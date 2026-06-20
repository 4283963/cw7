package snowflake

import (
	"errors"
	"sync"
	"time"
)

const (
	epoch           = int64(1704067200000)
	workerIDBits    = uint(5)
	datacenterBits  = uint(5)
	sequenceBits    = uint(12)
	maxWorkerID     = int64(-1 ^ (-1 << workerIDBits))
	maxDatacenterID = int64(-1 ^ (-1 << datacenterBits))
	sequenceMask    = int64(-1 ^ (-1 << sequenceBits))
	workerIDShift   = sequenceBits
	datacenterShift = sequenceBits + workerIDBits
	timestampShift  = sequenceBits + workerIDBits + datacenterBits
)

type Snowflake struct {
	mu           sync.Mutex
	timestamp    int64
	workerID     int64
	datacenterID int64
	sequence     int64
}

func NewSnowflake(workerID, datacenterID int64) (*Snowflake, error) {
	if workerID < 0 || workerID > maxWorkerID {
		return nil, errors.New("worker ID out of range")
	}
	if datacenterID < 0 || datacenterID > maxDatacenterID {
		return nil, errors.New("datacenter ID out of range")
	}
	return &Snowflake{
		workerID:     workerID,
		datacenterID: datacenterID,
		sequence:     0,
		timestamp:    -1,
	}, nil
}

func (s *Snowflake) NextID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	if now < s.timestamp {
		now = s.timestamp
	}
	if now == s.timestamp {
		s.sequence = (s.sequence + 1) & sequenceMask
		if s.sequence == 0 {
			for now <= s.timestamp {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		s.sequence = 0
	}
	s.timestamp = now
	id := ((now - epoch) << timestampShift) |
		(s.datacenterID << datacenterShift) |
		(s.workerID << workerIDShift) |
		s.sequence
	return id
}
