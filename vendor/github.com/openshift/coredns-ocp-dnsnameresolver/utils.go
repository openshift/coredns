package ocp_dnsnameresolver

import (
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// isWildcard checks if the domain name is wildcard. The input should
// be a valid fqdn.
func isWildcard(dnsName string) bool {
	return strings.HasPrefix(dnsName, "*.")
}

// getWildcard converts a regular DNS name to a wildcard DNS name. The
// input should be a valid fqdn.
func getWildcard(dnsName string) string {
	if isWildcard(dnsName) {
		return dnsName
	}
	return "*" + dnsName[strings.Index(dnsName, "."):]
}

// isSameNextLookupTime checks if the existing next lookup time (existing last lookup time + existing ttl)
// and the current next lookup time (current time + current ttl) are within a margin of 5 seconds of each
// other.
func isSameNextLookupTime(existingLastLookupTime time.Time, existingTTL, currentTTL int32) bool {
	existingNextLookupTime := existingLastLookupTime.Add(time.Duration(existingTTL) * time.Second)
	currentNextLookupTime := time.Now().Add(time.Duration(currentTTL) * time.Second)
	cmpOpts := []cmp.Option{
		cmpopts.EquateApproxTime(5 * time.Second),
	}
	return cmp.Equal(currentNextLookupTime, existingNextLookupTime, cmpOpts...)
}
