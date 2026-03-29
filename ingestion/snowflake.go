package main

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Simple Snowflake-style ID: timestamp (ms) + monotonic sequence.
// Format: "<unix_ms>-<seq>"
// Good enough for Week 1; swap for a real Snowflake library in production.

var seq uint64

func NewSnowflakeID() string {
	s := atomic.AddUint64(&seq, 1)
	ms := time.Now().UnixMilli()
	return fmt.Sprintf("%d-%d", ms, s)
}