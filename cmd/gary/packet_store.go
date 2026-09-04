package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type workPacketProvenance struct {
	JobID      string `json:"job_id"`
	SHA256     string `json:"sha256"`
	Source     string `json:"source"`
	SizeBytes  int    `json:"size_bytes"`
	RecordedAt string `json:"recorded_at"`
}

// persistWorkPacket stores the exact authoritative input bytes before paid
// execution. Existing packet bytes are immutable: a repeated job/digest may
// reuse them, but contradictory content at the same identity fails closed.
func persistWorkPacket(resultsDir string, m *workManifest) error {
	if m == nil || m.ID == "" || len(m.PacketBytes) == 0 || m.PacketDigest == "" {
		return fmt.Errorf("incomplete work packet provenance")
	}
	digest := sha256.Sum256(m.PacketBytes)
	if m.PacketDigest != fmt.Sprintf("%x", digest[:]) {
		return fmt.Errorf("packet digest does not match authoritative bytes")
	}
	shortDigest := m.PacketDigest
	if len(shortDigest) > 12 {
		shortDigest = shortDigest[:12]
	}
	dir := filepath.Join(resultsDir, "packets", shortSimpleID(m.ID, 80)+"-"+shortDigest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create packet store: %w", err)
	}
	packetPath := filepath.Join(dir, "packet.raw")
	if existing, err := os.ReadFile(packetPath); err == nil {
		if !bytes.Equal(existing, m.PacketBytes) {
			return fmt.Errorf("stored packet bytes conflict with authoritative packet")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read stored packet: %w", err)
	} else if err := os.WriteFile(packetPath, m.PacketBytes, 0o444); err != nil {
		return fmt.Errorf("write packet: %w", err)
	}

	provenancePath := filepath.Join(dir, "provenance.json")
	if existing, err := os.ReadFile(provenancePath); err == nil {
		var p workPacketProvenance
		if json.Unmarshal(existing, &p) != nil || p.JobID != m.ID || p.SHA256 != m.PacketDigest || p.SizeBytes != len(m.PacketBytes) {
			return fmt.Errorf("stored packet provenance conflicts with authoritative packet")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read packet provenance: %w", err)
	}

	p := workPacketProvenance{
		JobID:      m.ID,
		SHA256:     m.PacketDigest,
		Source:     m.PacketSource,
		SizeBytes:  len(m.PacketBytes),
		RecordedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("encode packet provenance: %w", err)
	}
	if err := os.WriteFile(provenancePath, data, 0o444); err != nil {
		return fmt.Errorf("write packet provenance: %w", err)
	}
	return nil
}
