module synapsex

go 1.23.0

// Phase 1 only initializes the Go module. Third-party adapter dependencies
// are added when their concrete implementations are introduced.

require github.com/lib/pq v1.10.9
