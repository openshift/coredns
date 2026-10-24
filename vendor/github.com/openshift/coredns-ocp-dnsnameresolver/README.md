# OpenShift DNSNameResolver

## Name

`OCP DNSNameResolver` - updates the status of the [DNSNameResolver](https://github.com/openshift/api/blob/master/network/v1alpha1/types_dnsnameresolver.go)
custom resources with IP addresses of matching resolved DNS names.

## Description

The spec of `DNSNameResolver` custom resource takes as input a DNS name. The DNS name can be either a regular or a wildcard DNS name. The plugin intercepts
the DNS lookups for the DNS records of type A/AAAA and matches them with the DNS names used in the `DNSNameResolver` CRs. The plugin updates the status of the
corresponding CRs with the IP addresses of the matching DNS names.

The plugin only adds any new IP address which are not already added to the status of the corresponding CRs, or updates the TTL and the last lookup time of
the existing IP addresses whose next lookup time has changed. The plugin does not remove any IP address from the list of IP addreses associated to a DNS
name from the status of the `DNSNameResolver` CRs. 

The plugin increments the `ResolutionFailures` field of the corresponding DNS name if the DNS lookup fails. If the DNS lookup for the DNS name fails
consecutively and the value of the `ResolutionFailures` field becomes greater than or equal to the plugin's configured `failureThreshold` value, and
if the TTL of all the associated IP addresses have expired, then the resolved name entry of the DNS name is removed from the status of the `DNSNameResolver`
CR. If the resolved name entry is not getting removed, then the IP addresses whose TTLs have expired or about to expire are set to the  plugin's configured
`minTTL` value and the last lookup time is set to current time.

The prerequisite for enabling this plugin are:
- Adding the `DNSNameResolver` CRD to the Kubernetes API.
- Adding `list` and `watch` permissions on the `DNSNameResolver` resources and `update` permission on the `DNSNameResolver/status` resource. These
permissions should be added to the serviceaccount used to deploy CoreDNS in a cluster.

NOTE: When adding the plugin to the `plugin.cfg` file in CoreDNS, care should be taken to place it before the plugins which will do the actual resolution of
the DNS names that will be used in the DNSNameResolver custom resources (eg. forward plugin). This will ensure that the plugin can intercept the DNS request
and response in the plugin chain.

## Syntax

```
ocp_dnsnameresolver {
    [namespaces NAMESPACE..]
    [minTTL MINTTL]
    [failureThreshold FAILURE_THRESHOLD]
}
```

- `namespaces` specifies those namespaces in which the `DNSNameResolver` custom resources will be monitored. When this option is omitted then `DNSNameResolver`
custom resource of all namespaces will be monitored.
- `minTTL` specifies the TTL value in seconds to be used for an IP address when the TTL in the DNS lookup response is zero OR when a DNS lookup fails and the
TTL of the IP address has expired. If the option is omitted then the default value of 5 seconds is used.
- `failureThreshold` specifies the number of consecutive DNS lookup failures for a DNS name until the details of the DNS name can be removed from the status
of a `DNSNameResolver` custom resource. However, the details of the DNS name will be removed only if the TTL of all the associated IP addresses have expired.
If the option is omitted then the default value of 5 is used.

## Examples

Enabling the `OCP DNSNameResolver` plugin with all defaults:

```
ocp_dnsnameresolver
```

Enabling the `OCP DNSNameResolver` plugin to monitor only a specific namespace:

```
ocp_dnsnameresolver {
    namespaces nsfoo
}
```

Enabling the `OCP DNSNameResolver` plugin with a different minimum TTL value:

```
ocp_dnsnameresolver {
    minTTL 10
}
```

Enabling the `OCP DNSNameResolver` plugin with a different failure threshold value:

```
ocp_dnsnameresolver {
    failureThreshold 10
}
```