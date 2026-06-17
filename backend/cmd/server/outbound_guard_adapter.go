package main

import (
	"github.com/daniil/floq/internal/ai/security"
	"github.com/daniil/floq/internal/outbound"
)

// outboundGuardAdapter bridges the context-free security.OutboundGuard to the
// outbound.SendGuard port, mapping the verdict struct to the port's
// (allowed, reason) shape. Lives in the composition root so neither the
// outbound nor the security package depends on the other.
type outboundGuardAdapter struct {
	g *security.OutboundGuard
}

func newOutboundGuardAdapter(g *security.OutboundGuard) outbound.SendGuard {
	return outboundGuardAdapter{g: g}
}

func (a outboundGuardAdapter) CheckBatch(size int) (bool, string) {
	d := a.g.CheckBatch(size)
	return d.Allowed, d.Reason
}

func (a outboundGuardAdapter) CheckRecipient(channel, recipient string) (bool, string) {
	d := a.g.CheckRecipient(channel, recipient)
	return d.Allowed, d.Reason
}
