package provider

import (
	"context"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-07-01/compute"
	"k8s.io/klog/v2"

	"sigs.k8s.io/cloud-provider-azure/pkg/cache"
)

type (
	VMSetStoreVMASEntry struct {
		ResourceGroup ResourceGroup
		VMAS          *compute.AvailabilitySet
		LastUpdated   time.Time
	}

	VMSetStoreVMAS struct {
		// map[ResourceGroup]map[VMASName]*VMSetStoreVMASEntry
		// value: sync.Map
		vmasByName *cache.TimedCache
	}
)

func newVMSetStoreVMAS(az *Cloud) (*VMSetStoreVMAS, error) {
	vmasByName, err := cache.NewTimedcache(time.Duration(az.Config.AvailabilitySetsCacheTTLInSeconds)*time.Second,
		func(resourceGroup ResourceGroup) (interface{}, error) {
			entry := &sync.Map{}

			availabilitySets, err := az.AvailabilitySetsClient.List(context.Background(), resourceGroup)
			if err != nil {
				klog.Errorf("AvailabilitySetsClient.List failed: %v", err)
				return nil, err.Error()
			}

			for i := range availabilitySets {
				availabilitySet := availabilitySets[i]
				if availabilitySet.Name == nil || *availabilitySet.Name == "" {
					klog.Warning("failed to get the name of AvailabilitySet")
					continue
				}
				entry.Store(*availabilitySet.Name, &VMSetStoreVMASEntry{
					VMAS:          &availabilitySet,
					ResourceGroup: resourceGroup,
					LastUpdated:   time.Now(),
				})
			}

			return entry, nil
		})
	if err != nil {
		return nil, err
	}

	return &VMSetStoreVMAS{
		vmasByName: vmasByName,
	}, nil
}
