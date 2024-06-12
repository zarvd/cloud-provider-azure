/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provider

import (
	"context"
	"fmt"
	"net/netip"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2022-07-01/network"
	"github.com/Azure/go-autorest/autorest/azure"

	azcache "sigs.k8s.io/cloud-provider-azure/pkg/cache"
	"sigs.k8s.io/cloud-provider-azure/pkg/consts"
	"sigs.k8s.io/cloud-provider-azure/pkg/provider/loadbalancer"
	"sigs.k8s.io/cloud-provider-azure/pkg/provider/loadbalancer/fnutil"
)

// listIPAddressesByPublicIPIDs retrieves a list of netip.Addr from the specified Azure.PublicIPAddress IDs.
func (az *Cloud) listIPAddressesByPublicIPIDs(ctx context.Context, pipIDs []string) ([]netip.Addr, error) {
	logger := klog.FromContext(ctx).WithName("listIPAddressesByPublicIPID")

	rv := make([]netip.Addr, 0, len(pipIDs))

	for _, id := range pipIDs {
		resourceID, err := azure.ParseResourceID(id)
		if err != nil {
			logger.Error(err, "Invalid resource ID of Public IP", "id", id)
			continue // FIXME fix unit tests that using dumb string as ID
		}

		logger.Info("Fetching Public IP", "pip-id", resourceID)
		pip, _, err := az.getPublicIPAddress(resourceID.ResourceGroup, resourceID.ResourceName, azcache.CacheReadTypeDefault)
		if err != nil {
			logger.Error(err, "Failed to fetch Public IP", "pip-id", resourceID)
			return nil, err
		}

		ip, err := netip.ParseAddr(*pip.IPAddress)
		if err != nil {
			logger.Error(err, "Failed to parse Public IP address", "ip", *pip.IPAddress)
			return nil, err
		}
		rv = append(rv, ip)
	}

	return rv, nil
}

func filterServicesByIngressIPs(services []*v1.Service, ips []netip.Addr) []*v1.Service {
	targetIPs := fnutil.Map(func(ip netip.Addr) string { return ip.String() }, ips)

	return fnutil.Filter(func(svc *v1.Service) bool {

		ingressIPs := fnutil.Map(func(ing v1.LoadBalancerIngress) string { return ing.IP }, svc.Status.LoadBalancer.Ingress)

		ingressIPs = fnutil.Filter(func(ip string) bool { return ip != "" }, ingressIPs)

		return len(fnutil.Intersection(ingressIPs, targetIPs)) > 0
	}, services)
}

func filterServicesByDisableFloatingIP(services []*v1.Service) []*v1.Service {
	return fnutil.Filter(func(svc *v1.Service) bool {
		return consts.IsK8sServiceDisableLoadBalancerFloatingIP(svc)
	}, services)
}

// listSharedIPPortMapping lists the shared IP port mapping for the service excluding the service itself.
// There are scenarios where multiple services share the same public IP,
// and in order to clean up the security rules, we need to know the port mapping of the shared IP.
func (az *Cloud) listSharedIPPortMapping(
	ctx context.Context,
	svc *v1.Service,
	publicIPs []network.PublicIPAddress,
) (map[network.SecurityRuleProtocol][]int32, error) {
	var (
		logger = klog.FromContext(ctx).WithName("listSharedIPPortMapping")
		rv     = make(map[network.SecurityRuleProtocol][]int32)
	)

	var services []*v1.Service
	{
		var err error
		logger.Info("Listing all services")
		services, err = az.serviceLister.List(labels.Everything())
		if err != nil {
			logger.Error(err, "Failed to list all services")
			return nil, fmt.Errorf("list all services: %w", err)
		}
		logger.Info("Listed all services", "num-all-services", len(services))

		// Filter services by ingress IPs or backend node pool IPs (when disable floating IP)
		if consts.IsK8sServiceDisableLoadBalancerFloatingIP(svc) {
			logger.Info("Filter service by disableFloatingIP")
			services = filterServicesByDisableFloatingIP(services)
		} else {
			logger.Info("Filter service by external IPs")
			pipIDs := fnutil.Map(func(ip network.PublicIPAddress) string { return ptr.Deref(ip.ID, "") }, publicIPs)
			ips, err := az.listIPAddressesByPublicIPIDs(ctx, pipIDs)
			if err != nil {
				logger.Error(err, "Failed to list external IPs of services")
				return nil, err
			}
			services = filterServicesByIngressIPs(services, ips)
		}
	}
	logger.Info("Filtered services", "num-filtered-services", len(services))

	for _, s := range services {
		logger.V(4).Info("iterating service", "service", s.Name, "namespace", s.Namespace)
		if svc.Namespace == s.Namespace && svc.Name == s.Name {
			// skip the service itself
			continue
		}

		portsByProtocol, err := loadbalancer.SecurityRuleDestinationPortsByProtocol(s)
		if err != nil {
			return nil, fmt.Errorf("fetch security rule dst ports for %s: %w", s.Name, err)
		}

		for protocol, ports := range portsByProtocol {
			rv[protocol] = append(rv[protocol], ports...)
		}
	}

	logger.V(4).Info("retain port mapping", "port-mapping", rv)

	return rv, nil
}
