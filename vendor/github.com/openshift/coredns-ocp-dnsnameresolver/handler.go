package ocp_dnsnameresolver

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/request"
	ocpnetworkapiv1alpha1 "github.com/openshift/api/network/v1alpha1"
	ocpnetworkv1alpha1lister "github.com/openshift/client-go/network/listers/network/v1alpha1"

	"github.com/miekg/dns"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
)

const (
	ConditionDegraded = "Degraded"
)

var rcodeMessage = map[int]string{
	// Message Response Codes, see https://www.iana.org/assignments/dns-parameters/dns-parameters.xhtml
	dns.RcodeSuccess:        "No Error",
	dns.RcodeFormatError:    "Format Error",
	dns.RcodeServerFailure:  "Server Failure",
	dns.RcodeNameError:      "Non-Existent Domain",
	dns.RcodeNotImplemented: "Not Implemented",
	dns.RcodeRefused:        "Query Refused",
	dns.RcodeYXDomain:       "Name Exists when it should not",
	dns.RcodeYXRrset:        "RR Set Exists when it should not",
	dns.RcodeNXRrset:        "RR Set that should exist does not",
	dns.RcodeNotAuth:        "Server Not Authoritative for zone",
	dns.RcodeNotZone:        "Name not contained in zone",
	dns.RcodeBadSig:         "TSIG Signature Failure", // Also known as RcodeBadVers, see RFC 6891
	dns.RcodeBadKey:         "Key not recognized",
	dns.RcodeBadTime:        "Signature out of time window",
	dns.RcodeBadMode:        "Bad TKEY Mode",
	dns.RcodeBadName:        "Duplicate key name",
	dns.RcodeBadAlg:         "Algorithm not supported",
	dns.RcodeBadTrunc:       "Bad Truncation",
	dns.RcodeBadCookie:      "Bad/missing Server Cookie",
}

// ServeDNS implements the plugin.Handler interface.
func (resolver *OCPDNSNameResolver) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	// Get the DNS name from the DNS lookup request.
	qname := strings.ToLower(state.QName())

	var regularDnsInfo, wildcardDnsInfo namespaceDNSInfo
	var regularDNSExists, wildcardDNSExists bool

	// Check if the query was for a wildcard DNS name or a regular DNS name.
	if isWildcard(qname) {
		// Get the wildcard DNS name info, if it exists.
		wildcardDnsInfo, wildcardDNSExists = resolver.wildcardDNSInfo[qname]
	} else {
		// Get the regular DNS name info, if it exists.
		regularDnsInfo, regularDNSExists = resolver.regularDNSInfo[qname]

		// Get the corresponding wildcard DNS name for the reguar DNS name.
		wildcard := getWildcard(qname)
		// Get the wildcard DNS name info, if it exists.
		wildcardDnsInfo, wildcardDNSExists = resolver.wildcardDNSInfo[wildcard]
	}

	// If neither regular DNS name info nor wildcard DNS name info exists for the DNS name
	// then return the response received from the plugin chain.
	if !regularDNSExists && !wildcardDNSExists {
		return plugin.NextOrFailure(resolver.Name(), resolver.Next, ctx, w, r)
	}

	// Record response to get status code and size of the reply.
	rw := dnstest.NewRecorder(w)

	// Get the response for the DNS lookup from the plugin chain.
	status, err := plugin.NextOrFailure(resolver.Name(), resolver.Next, ctx, rw, r)

	// Get the IP addresses and the corresponding TTLs in a map. Only A and AAAA type DNS records
	// are considered.
	ipTTLs := make(map[string]int32)
	for _, answer := range rw.Msg.Answer {
		switch state.QType() {
		case dns.TypeA:
			if rec, ok := answer.(*dns.A); ok {
				ttl := int32(rec.Hdr.Ttl)
				if ttl == 0 {
					ttl = resolver.minimumTTL
				}
				ipTTLs[rec.A.String()] = ttl
			}
		case dns.TypeAAAA:
			if rec, ok := answer.(*dns.AAAA); ok {
				ttl := int32(rec.Hdr.Ttl)
				if ttl == 0 {
					ttl = resolver.minimumTTL
				}
				ipTTLs[rec.AAAA.String()] = ttl
			}
		default:
			return status, err
		}
	}

	// Check if the DNS lookup is unsuccessful or an error is encountered during the lookup.
	if status != dns.RcodeSuccess || err != nil {
		// WaitGroup variable used to wait for the completion of update of DNSNameResolver CRs
		// corresponding to the regular and the wildcard DNS names.
		var wg sync.WaitGroup

		// If regular DNS name info exists then update the corresponding DNSNameResolver CR for
		// the DNS lookup failure.
		if regularDNSExists {
			wg.Add(1)
			go func() {
				defer wg.Done()
				resolver.updateResolvedNamesFailure(ctx, regularDnsInfo, qname, status)
			}()
		}

		// If wildcard DNS name info exists then update the corresponding DNSNameResolver CR for
		// the DNS lookup failure.
		if wildcardDNSExists {
			wg.Add(1)
			go func() {
				defer wg.Done()
				resolver.updateResolvedNamesFailure(ctx, wildcardDnsInfo, qname, status)
			}()
		}

		// Wait for the goroutines to complete.
		wg.Wait()

		// Return the response received from the plugin chain.
		return status, err
	}

	// If no IP address is Return the response received from the plugin chain.
	if len(ipTTLs) == 0 {
		return status, err
	}

	// WaitGroup variable used to wait for the completion of update of DNSNameResolver CRs
	// corresponding to the regular and the wildcard DNS names.
	var wg sync.WaitGroup

	// If regular DNS name info exists then update the corresponding DNSNameResolver CR for
	// the successful DNS lookup.
	if regularDNSExists {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resolver.updateResolvedNamesSuccess(ctx, regularDnsInfo, qname, ipTTLs)

		}()
	}

	// If wildcard DNS name info exists then update the corresponding DNSNameResolver CR for
	// the successful DNS lookup.
	if wildcardDNSExists {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resolver.updateResolvedNamesSuccess(ctx, wildcardDnsInfo, qname, ipTTLs)
		}()
	}

	// Wait for the goroutines to complete.
	wg.Wait()

	// Return the response received from the plugin chain.
	return status, err
}

// Name implements the Handler interface.
func (resolver *OCPDNSNameResolver) Name() string { return pluginName }

// updateResolvedNamesSuccess updates the ResolvedNames field of the corresponding DNSNameResolver object when DNS lookup is successfully completed.
func (resolver *OCPDNSNameResolver) updateResolvedNamesSuccess(
	ctx context.Context,
	namespaceDNS namespaceDNSInfo,
	dnsName string,
	ipTTLs map[string]int32,
) {
	// WaitGroup variable used to wait for the completion of update of DNSNameResolver CRs
	// for the same DNS name in different namespaces.
	var wg sync.WaitGroup

	// Iterate through the namespaces and the corresponding DNSNameResolver object names.
	for namespace, objName := range namespaceDNS {
		wg.Add(1)

		// Each update is performed in separate goroutine.
		go func(namespace string, objName string) {
			defer wg.Done()

			previousResourceVersion := "0"
			// Retry the update of the DNSNameResolver object if there's a conflict during the update.
			retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				var (
					resolverObj     *ocpnetworkapiv1alpha1.DNSNameResolver
					resourceVersion string
					err             error
				)

				// Fetch the DNSNameResolver object using the lister first.
				resolverObj, err = ocpnetworkv1alpha1lister.NewDNSNameResolverLister(
					resolver.dnsNameResolverInformer.GetIndexer()).DNSNameResolvers(namespace).Get(objName)
				if err != nil {
					return err
				}
				resourceVersion = resolverObj.GetResourceVersion()
				// Check if the current and the previous resource version match or not.
				if resourceVersion == previousResourceVersion {
					listerResourceVersion := resourceVersion
					// This indicates that there was a conflict and the lister has not caught up.
					// So fetch the DNSNameResolver object using the client.
					resolverObj, err = resolver.ocpNetworkClient.DNSNameResolvers(namespace).Get(ctx, objName, metav1.GetOptions{})
					if err != nil {
						return err
					}
					resourceVersion = resolverObj.GetResourceVersion()
					log.Infof("lister was stale at resourceVersion=%v, live get showed resourceVersion=%v", listerResourceVersion, resourceVersion)
				}
				previousResourceVersion = resourceVersion

				// Make a copy of the object. All the updates will be applied to the copied object.
				newResolverObj := resolverObj.DeepCopy()
				// Get the DNS name from the spec.name field.
				specDNSName := string(newResolverObj.Spec.Name)
				// Get the current time.
				currentTime := metav1.NewTime(time.Now())

				// existingIndex gives the index of the of the resolved name corresponding to the
				// DNS name, which is currently being looked up, if it exists.
				var existingIndex int
				// foundResolvedName indicates whether the resolved name corresponding to the DNS name,
				// which is currently being looked up, was found or not.
				foundResolvedName := false
				// matchedWildcard indicates whether the current regular DNS name being looked up
				// completely matches the resolved name entry of the wildcard DNS name corresponding
				// to the DNSNameResolver object. For the match to succeed, the IP addresses associated
				// with the regular DNS name should be contained in the list of IP addresses present
				// in the resolved name entry of the wildcard DNS name. If the current DNS name being
				// looked up is regular or the DNSNameResolver object corresponds to a regular DNS
				// name then the value of matchedWildcard will be false.
				matchedWildcard := false
				// statusUpdated indicates whether the status of the DNSNameResolver object should
				// be updated or not.
				statusUpdated := false
				// indicesMatchingWildcard contains the existing resolved name entries of the regular
				// DNS names completely matching that of the wildcard DNS name's resolved name entry.
				// This map will contain the indices only when the DNSNameResolver object is for a
				// wildcard DNS name and the DNS name lookup is also for the same DNS name.
				indicesMatchingWildcard := []int{}

				// Iterate through each resolved name present in the status of the DNSNameResolver object.
				//
				// NOTE: The resolved name for a wildcard DNS name, if it exists, will always be the first one in the list of
				// resolved names in the status of the DNSNameResolver object corresponding to the wildcard DNS name.
				for index, resolvedName := range newResolverObj.Status.ResolvedNames {
					if isWildcard(specDNSName) && !isWildcard(dnsName) && strings.EqualFold(string(resolvedName.DNSName), specDNSName) {
						// Case 1: When the DNSNameResolver object is for a wildcard DNS name, the lookup is for a regular DNS name
						// which matches the wildcard DNS name, and the current resolved name is for the wildcard DNS name.

						// Check if the regular DNS name completely matches the resolved name entry of the wildcard DNS name.
						// The regular DNS name will completely match the wildcard DNS name if all the IP addresses that are received
						// in the response of the DNS name lookup already exists in the wildcard DNS name's resolved name field, the
						// corresponding next lookup time of the IP addresses also matches.
						matchedWildcard = isMatchingResolvedName(ipTTLs, resolvedName)
					} else if strings.EqualFold(string(resolvedName.DNSName), dnsName) {
						// Case 2: When the DNS name which is being resolved matches the current resolved name. This is applicable
						// for DNSNameResolver objects for both the regular and wildcard DNS names.

						// If matchedWildcard is set to true, then the DNS lookup is for a regular DNS name and the DNSNameResolver
						// object is corresponding to a wildcard DNS name. The IP addresses that are received in the response of the
						// DNS name lookup already exists in the wildcard DNS name's resolved name field. However, as the regular
						// DNS name's resolved name also exists, it means that some of the existing IP addresses associated with the
						// regular DNS name do not match with the IP addresses associated with the wildcard DNS name. Thus,
						// matchedWildcard is set to false.
						matchedWildcard = false

						// As the resolved name for the DNS name being looked up is found, set foundResolvedName to true.
						foundResolvedName = true
						// Set existingIndex to the current value of the index variable to indicate the index at which the
						// resolved name corresponding to the DNS name exists.
						existingIndex = index

						// If any of the IP address already exists, it's corresponding TTL and last lookup time will be updated if
						// the next lookup time (TTL + last lookup time) has changed.
						//
						// The IP addresses which do not already exist, will be added to the existing resolvedAddresses list.
						//
						// The resolutionFailures field will be set to zero. If the conditions field is not set or if the existing
						// status of the "Degraded" condition is not false, then the status of the condition will be set to false,
						// reason and message will be set to corresponding to that of success rcode.
						statusUpdated = addUpdateResolvedNameIPTTLs(index, ipTTLs, currentTime, newResolverObj)
					} else if isWildcard(dnsName) {
						// Case 3: When the DNSNameResolver object is for a wildcard DNS name, the lookup is also for the wildcard DNS name,
						// and the current resolved name is for a regular DNS name which matches the wildcard DNS name.

						// Check if the resolved name for the regular DNS name completely matches the wildcard DNS name corresponding to the
						// DNSNameResolver object, along with the IP addresses. If it matches then add the index of the resolved name entry
						// of the regular DNS name to the indicesMatchingWildcard map.
						if isRegularMatchingWildcardResolvedName(foundResolvedName, newResolverObj, resolvedName, ipTTLs, currentTime) {
							indicesMatchingWildcard = append(indicesMatchingWildcard, index)
						}
					}

					// Skip all the remaining resolved names, if the DNS lookup is for a regular DNS name, the DNSNameResolver object
					// is corresponding to a wildcard DNS name, the regular DNS name's resolved name field is already found, and the
					// check for the complete match of the regular DNS name with the wildcard DNS name has already been performed.
					if !isWildcard(dnsName) && isWildcard(specDNSName) && foundResolvedName && index > 0 {
						break
					}
				}

				// If the DNS lookup is for a wildcard DNS name, then remove the existing resolved name entries of the regular DNS names
				// completely matching that of the wildcard DNS name's resolved name entry.
				if isWildcard(dnsName) {
					isRemoved := removeResolvedNames(indicesMatchingWildcard, newResolverObj)
					statusUpdated = statusUpdated || isRemoved
				}

				if !isWildcard(dnsName) && matchedWildcard {
					// Remove the regular DNS name's resolved name entry which completely matches that of the wildcard DNS name's resolved name.

					indexList := []int{}
					// Add the index of the regular DNS name's resolved name entry to the indexList, if it is found.
					if foundResolvedName {
						indexList = append(indexList, existingIndex)
					}
					isRemoved := removeResolvedNames(indexList, newResolverObj)
					statusUpdated = statusUpdated || isRemoved
				} else if !foundResolvedName {
					// Add the resolved name entry for the DNS name (applies to both regular and wildcard DNS names) if the entry is not found.
					addResolvedName(dnsName, currentTime, ipTTLs, newResolverObj)
					statusUpdated = true
				}

				// If there are no changes to the status of the DNSNameResolver object then skip the update status call.
				if !statusUpdated {
					return nil
				}

				// Update the status of the DNSNameResolver object.
				_, err = resolver.ocpNetworkClient.DNSNameResolvers(namespace).UpdateStatus(ctx, newResolverObj, metav1.UpdateOptions{})
				return err
			})
			if retryErr != nil {
				log.Errorf("Encountered error while updating status of DNSNameResolver object: %v", retryErr)
			}
		}(namespace, objName)
	}

	// Wait for the goroutines for each namespace to complete.
	wg.Wait()
}

// isMatchingResolvedName checks if all the IP addresses in the ipTTLs map are contained
// in the resolved addresses of the resolved name and the corresponding next lookup times of
// IP addresses also match.
func isMatchingResolvedName(
	ipTTLs map[string]int32,
	resolvedName ocpnetworkapiv1alpha1.DNSNameResolverResolvedName,
) bool {

	matchedIPTTLs := sets.New[string]()
	matched := true

	// Iterate through the associated IP addresses of the wildcard DNS name and check if all the IP addresses
	// received in the response of the DNS lookup of the regular DNS name completely match them.
	for _, resolvedAddress := range resolvedName.ResolvedAddresses {
		ttl, exists := ipTTLs[resolvedAddress.IP]
		if !exists {
			matched = false
			break
		}
		if !isSameNextLookupTime(resolvedAddress.LastLookupTime.Time, resolvedAddress.TTLSeconds, ttl) {
			matched = false
			break
		}
		matchedIPTTLs.Insert(resolvedAddress.IP)
	}
	if matched && len(ipTTLs) != matchedIPTTLs.Len() {
		matched = false
	}
	return matched
}

// addUpdateResolvedNameIPTTLs adds the IP addresses to the resolved name's resolved addresses which currently does not exist
// in the resolved addresses. If an IP address already exists but the corresponding next lookup time of the IP address has
// changed then it updates the TTL of the IP address.
func addUpdateResolvedNameIPTTLs(
	index int,
	ipTTLs map[string]int32,
	currentTime metav1.Time,
	resolverObj *ocpnetworkapiv1alpha1.DNSNameResolver,
) bool {

	matchedIPTTLs := sets.New[string]()
	statusUpdated := false

	// Iterate through the existing associated IP addresses of the DNS name and update the corresponding TTL and last
	// lookup if the next lookup time has changed.
	for i, resolvedAddress := range resolverObj.Status.ResolvedNames[index].ResolvedAddresses {
		if ttl, matched := ipTTLs[resolvedAddress.IP]; matched {
			if !isSameNextLookupTime(resolvedAddress.LastLookupTime.Time, resolvedAddress.TTLSeconds, ttl) {
				resolverObj.Status.ResolvedNames[index].ResolvedAddresses[i].TTLSeconds = ttl
				resolverObj.Status.ResolvedNames[index].ResolvedAddresses[i].LastLookupTime = currentTime.DeepCopy()
				statusUpdated = true
			}
			matchedIPTTLs.Insert(resolvedAddress.IP)
		}
	}

	// Append the IP addresses which are not already available in the list of resolvedAddresses of the DNS name.
	for ip, ttl := range ipTTLs {
		if !matchedIPTTLs.Has(ip) {
			resolvedAddress := ocpnetworkapiv1alpha1.DNSNameResolverResolvedAddress{
				IP:             ip,
				TTLSeconds:     ttl,
				LastLookupTime: currentTime.DeepCopy(),
			}
			resolverObj.Status.ResolvedNames[index].ResolvedAddresses =
				append(resolverObj.Status.ResolvedNames[index].ResolvedAddresses, resolvedAddress)
			statusUpdated = true
		}
	}

	// Set the resolutionFailures field to zero.
	resolverObj.Status.ResolvedNames[index].ResolutionFailures = 0

	// If the conditions field is not set or if the existing status of the "Degraded" condition is not false,
	// then the status of the condition will be set to false, reason and message will be set to corresponding
	// to that of success rcode.
	if len(resolverObj.Status.ResolvedNames[index].Conditions) == 0 {
		resolverObj.Status.ResolvedNames[index].Conditions = []metav1.Condition{
			{
				Type:               ConditionDegraded,
				Status:             metav1.ConditionFalse,
				LastTransitionTime: currentTime,
				Reason:             dns.RcodeToString[dns.RcodeSuccess],
				Message:            rcodeMessage[dns.RcodeSuccess],
			},
		}
		statusUpdated = true
	} else if resolverObj.Status.ResolvedNames[index].Conditions[0].Status != metav1.ConditionFalse {
		resolverObj.Status.ResolvedNames[index].Conditions[0].Status = metav1.ConditionFalse
		resolverObj.Status.ResolvedNames[index].Conditions[0].LastTransitionTime = currentTime
		resolverObj.Status.ResolvedNames[index].Conditions[0].Reason = dns.RcodeToString[dns.RcodeSuccess]
		resolverObj.Status.ResolvedNames[index].Conditions[0].Message = rcodeMessage[dns.RcodeSuccess]
		statusUpdated = true
	}

	return statusUpdated
}

// isRegularMatchingWildcardResolvedName checks if all the IP addresses associated with the regular DNS name are also
// associated with the wildcard DNS name and the corresponding next lookup time of the IP addresses also matches. If
// so then the index of the regular DNS name should be stored as it completely matches the wildcard DNS name.
func isRegularMatchingWildcardResolvedName(
	foundDNSName bool,
	newResolverObj *ocpnetworkapiv1alpha1.DNSNameResolver,
	resolvedName ocpnetworkapiv1alpha1.DNSNameResolverResolvedName,
	ipTTLs map[string]int32,
	currentTime metav1.Time,
) bool {
	wildcardIPTTLs := make(map[string]int32)

	// If the wildcard DNS name's resolved name field exists, then it would be already found, as it will be the first in
	// the list of resolved names.
	if foundDNSName {
		// Add all the existing IP addresses associated with the wildcard DNS name with the updated TTLs.
		for _, resolvedAddress := range newResolverObj.Status.ResolvedNames[0].ResolvedAddresses {
			wildcardIPTTLs[resolvedAddress.IP] = resolvedAddress.TTLSeconds - int32(currentTime.Time.Sub(resolvedAddress.LastLookupTime.Time).Seconds())
		}
	}

	// Add all the current IP addresses associated with the wildcard DNS name with the corresponding TTLs.
	for ip, ttl := range ipTTLs {
		wildcardIPTTLs[ip] = ttl
	}

	// Iterate through all the associated IP addresses of the regular DNS name and check if all of them
	// are also associated with the wildcard DNS name, and the corresponding next lookup time of the
	// IP addresses also matches.
	addIndex := true
	for _, resolvedAddress := range resolvedName.ResolvedAddresses {
		ttl, matched := wildcardIPTTLs[resolvedAddress.IP]
		if !matched {
			addIndex = false
			break
		}
		if !isSameNextLookupTime(resolvedAddress.LastLookupTime.Time, resolvedAddress.TTLSeconds, ttl) {
			addIndex = false
			break
		}
	}
	return addIndex
}

// removeResolvedNames removes the resolved names from the status of the DNSNameResolver object
// using the indicesToRemove slice.
func removeResolvedNames(
	indicesToRemove []int,
	resolverObj *ocpnetworkapiv1alpha1.DNSNameResolver,
) bool {
	count := 0
	for index := range indicesToRemove {
		if len(resolverObj.Status.ResolvedNames) == index-count+1 {
			resolverObj.Status.ResolvedNames = resolverObj.Status.ResolvedNames[:index-count]
		} else {
			resolverObj.Status.ResolvedNames = append(resolverObj.Status.ResolvedNames[:index-count], resolverObj.Status.ResolvedNames[index-count+1:]...)
		}
		count++
	}

	return count != 0
}

// addResolvedName adds a new resolved name for the dnsName to the list of existing resolved names.
// If the resolved name is for a wildcard DNS name then the entry will be added to the beginning of
// the list, otherwise it will be added to the end.
func addResolvedName(
	dnsName string,
	currentTime metav1.Time,
	ipTTLs map[string]int32,
	newResolverObj *ocpnetworkapiv1alpha1.DNSNameResolver,
) {
	// Create the resolved name entry.
	resolvedName := ocpnetworkapiv1alpha1.DNSNameResolverResolvedName{
		DNSName:            ocpnetworkapiv1alpha1.DNSName(dnsName),
		ResolutionFailures: 0,
		Conditions: []metav1.Condition{
			{
				Type:               ConditionDegraded,
				Status:             metav1.ConditionFalse,
				LastTransitionTime: currentTime,
				Reason:             dns.RcodeToString[dns.RcodeSuccess],
				Message:            rcodeMessage[dns.RcodeSuccess],
			},
		},
	}

	// Add the IP addresses to the resolved name entry.
	for ip, ttl := range ipTTLs {
		resolvedAddress := ocpnetworkapiv1alpha1.DNSNameResolverResolvedAddress{
			IP:             ip,
			TTLSeconds:     ttl,
			LastLookupTime: currentTime.DeepCopy(),
		}
		resolvedName.ResolvedAddresses = append(resolvedName.ResolvedAddresses, resolvedAddress)
	}

	if isWildcard(dnsName) {
		// Add the resolved name entry for the wildcard DNS name at the beginning of the list of resolved names.
		newResolverObj.Status.ResolvedNames = append([]ocpnetworkapiv1alpha1.DNSNameResolverResolvedName{resolvedName}, newResolverObj.Status.ResolvedNames...)
	} else {
		// Add the resolved name entry for the regular DNS name at the end of the list of resolved names.
		newResolverObj.Status.ResolvedNames = append(newResolverObj.Status.ResolvedNames, resolvedName)
	}
}

// updateResolvedNamesFailure updates the ResolvedNames field of the corresponding DNSNameResolver object.
func (resolver *OCPDNSNameResolver) updateResolvedNamesFailure(ctx context.Context, namespaceDNS namespaceDNSInfo, dnsName string, rcode int) {
	// WaitGroup variable used to wait for the completion of update of DNSNameResolver CRs
	// for the same DNS name in different namespaces.
	var wg sync.WaitGroup

	// Iterate through the namespaces and the corresponding DNSNameResolver object names.
	for namespace, objName := range namespaceDNS {
		wg.Add(1)

		// Each update is performed in separate goroutine.
		go func(namespace string, objName string) {
			defer wg.Done()

			previousResourceVersion := "0"
			// Retry the update of the DNSNameResolver object if there's a conflict during the update.
			retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				var (
					resolverObj     *ocpnetworkapiv1alpha1.DNSNameResolver
					resourceVersion string
					err             error
				)
				// Fetch the DNSNameResolver object using the lister first.
				resolverObj, err = ocpnetworkv1alpha1lister.NewDNSNameResolverLister(
					resolver.dnsNameResolverInformer.GetIndexer()).DNSNameResolvers(namespace).Get(objName)
				if err != nil {
					return err
				}
				resourceVersion = resolverObj.GetResourceVersion()
				// Check if the current and the previous resource version match or not.
				if resourceVersion == previousResourceVersion {
					listerResourceVersion := resourceVersion
					// This indicates that there was a conflict and the lister has not caught up.
					// So fetch the DNSNameResolver object using the client.
					resolverObj, err = resolver.ocpNetworkClient.DNSNameResolvers(namespace).Get(ctx, objName, metav1.GetOptions{})
					if err != nil {
						return err
					}
					resourceVersion = resolverObj.GetResourceVersion()
					log.Infof("lister was stale at resourceVersion=%v, live get showed resourceVersion=%v", listerResourceVersion, resourceVersion)
				}
				previousResourceVersion = resourceVersion

				// Make a copy of the object. All the updates will be applied to the copied object.
				newResolverObj := resolverObj.DeepCopy()
				// Get the current time.
				currentTime := metav1.NewTime(time.Now())

				// existingIndex gives the index of the of the resolved name corresponding to the
				// DNS name, which is currently being looked up, if it exists.
				var existingIndex int
				// foundResolvedName indicates whether the resolved name corresponding to the DNS name,
				// which is currently being looked up, was found or not.
				foundResolvedName := false
				// removeResolvedName indicates whether the resolved name entry corresponding to the
				// DNS name being looked up needs to be removed or not. The value of removeResolvedName
				// will be true if the value of resolutionFailures of the resolved name is greater than
				// equal to the configured failure threshold, otherwise the value of removeResolvedName
				// will be false.
				removeResolvedName := false
				// statusUpdated indicates whether the status of the DNSNameResolver object should
				// be updated or not.
				statusUpdated := false

				// Iterate through each resolved name present in the status of the DNSNameResolver object.
				for index, resolvedName := range newResolverObj.Status.ResolvedNames {

					// Check if the DNS name which is being resolved matches the current resolved name.
					if strings.EqualFold(string(resolvedName.DNSName), dnsName) {

						// As the resolved name for the DNS name being looked up is found, set foundResolvedName to true.
						foundResolvedName = true
						// Set existingIndex to the current value of the index variable to indicate the index at which the
						// resolved name corresponding to the DNS name exists.
						existingIndex = index

						// Check whether the resolved name for the DNS name needs to be removed or not. If not, then update
						// the resolved name entry to reflect the failure in DNS resolution.
						removeResolvedName, statusUpdated =
							checkAndUpdateResolvedName(index, newResolverObj, currentTime, resolver.failureThreshold, resolver.minimumTTL, rcode)
					}

					// Skip all the remaining resolved names, if the DNS name's resolved name is already found.
					if foundResolvedName {
						break
					}
				}

				if !foundResolvedName {
					// If the resolved name entry is not found then no update operation is required.
					return nil
				} else if removeResolvedName {
					// Remove the resolved name entry if the resolutionFailures field's value is greater than or equal
					// to the failure threshold.
					newResolverObj.Status.ResolvedNames = append(newResolverObj.Status.ResolvedNames[:existingIndex], newResolverObj.Status.ResolvedNames[existingIndex+1:]...)
					statusUpdated = true
				}

				// If there are no changes to the status of the DNSNameResolver object then skip the update status call.
				if !statusUpdated {
					return nil
				}

				// Update the status of the DNSNameResolver object.
				_, err = resolver.ocpNetworkClient.DNSNameResolvers(namespace).UpdateStatus(ctx, newResolverObj, metav1.UpdateOptions{})
				return err
			})
			if retryErr != nil {
				log.Errorf("Encountered error while updating status of DNSNameResolver object: %v", retryErr)
			}
		}(namespace, objName)
	}

	// Wait for the goroutines for each namespace to complete.
	wg.Wait()
}

// checkAndUpdateResolvedName checks whether the resolved name needs to be removed or not. If not, then the resolutionFailures
// of the resolved name is incremented by one, and the "Degraded" condition is set to true.
func checkAndUpdateResolvedName(
	index int,
	newResolverObj *ocpnetworkapiv1alpha1.DNSNameResolver,
	currentTime metav1.Time,
	failureThreshold int32,
	minimumTTL int32,
	rcode int,
) (removeResolvedName bool, statusUpdated bool) {

	// Check if the resolutionFailures of the resolved name is greater than or equal to the failure threshold.
	if newResolverObj.Status.ResolvedNames[index].ResolutionFailures >= failureThreshold {
		removeResolvedName = true

		// Iterate through each of the IP addresses associated to the DNS name and check if the corresponding TTL
		// has expired. The resolved name entry will only be removed if the TTLs of all the IP addresses have
		// expired.
		for _, resolvedAdress := range newResolverObj.Status.ResolvedNames[index].ResolvedAddresses {
			nextLookupTime := resolvedAdress.LastLookupTime.Time.Add(time.Duration(resolvedAdress.TTLSeconds) * time.Second)
			if nextLookupTime.After(currentTime.Time) {
				removeResolvedName = false
				break
			}
		}
	}

	// If the resolved name entry is not getting removed, then the IP addresses whose TTLs have expired or about
	// to expire should be set to the minimum TTL value and the last lookup time should be set to current time.
	// Additionally, the resolutionFailures field value should be incremented by 1. If the conditions field is not
	// set or if the existing status of the "Degraded" condition is not true, then the status of the condition
	// will be set to true, reason and message will be set to corresponding to that of corresponding failure rcode.
	if !removeResolvedName {
		// Iterate through the associated IP addresses of the resolved name, and update the TTLs and the last
		// lookup times of the IP addresses which have expired.
		for i, resolvedAdress := range newResolverObj.Status.ResolvedNames[index].ResolvedAddresses {
			nextLookupTime := resolvedAdress.LastLookupTime.Time.Add(time.Duration(resolvedAdress.TTLSeconds) * time.Second)
			if !nextLookupTime.After(currentTime.Time) ||
				isSameNextLookupTime(resolvedAdress.LastLookupTime.Time, resolvedAdress.TTLSeconds, 0) {
				newResolverObj.Status.ResolvedNames[index].ResolvedAddresses[i].TTLSeconds = minimumTTL
				newResolverObj.Status.ResolvedNames[index].ResolvedAddresses[i].LastLookupTime = &currentTime
				statusUpdated = true
			}
		}

		// Increment the resolutionFailures field value by 1.
		newResolverObj.Status.ResolvedNames[index].ResolutionFailures++

		// If the conditions field is not set or if the existing status of the "Degraded" condition is not true, then
		// the status of the condition will be set to true, reason and message will be set to corresponding to that
		// of corresponding failure rcode.
		if len(newResolverObj.Status.ResolvedNames[index].Conditions) == 0 {
			newResolverObj.Status.ResolvedNames[index].Conditions = []metav1.Condition{
				{
					Type:               ConditionDegraded,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: currentTime,
					Reason:             dns.RcodeToString[rcode],
					Message:            rcodeMessage[rcode],
				},
			}
		} else if newResolverObj.Status.ResolvedNames[index].Conditions[0].Status != metav1.ConditionTrue ||
			newResolverObj.Status.ResolvedNames[index].Conditions[0].Reason != dns.RcodeToString[rcode] {
			newResolverObj.Status.ResolvedNames[index].Conditions[0].Status = metav1.ConditionTrue
			newResolverObj.Status.ResolvedNames[index].Conditions[0].LastTransitionTime = currentTime
			newResolverObj.Status.ResolvedNames[index].Conditions[0].Reason = dns.RcodeToString[rcode]
			newResolverObj.Status.ResolvedNames[index].Conditions[0].Message = rcodeMessage[rcode]
		}
		statusUpdated = true
	}

	return removeResolvedName, statusUpdated
}
