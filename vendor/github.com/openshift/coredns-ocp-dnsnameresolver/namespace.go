package ocp_dnsnameresolver

// configuredNamespace returns true when the given namespace is specified in the
// `namespaces` configuration or if the `namespaces` configuration is omitted.
func (resolver *OCPDNSNameResolver) configuredNamespace(namespace string) bool {
	_, ok := resolver.namespaces[namespace]
	if len(resolver.namespaces) > 0 && !ok {
		return false
	}
	return true
}
