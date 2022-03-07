package provider

import (
	"context"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-07-01/compute"
	"github.com/Azure/go-autorest/autorest/to"
	"k8s.io/klog/v2"

	"sigs.k8s.io/cloud-provider-azure/pkg/cache"
)

type VMSetStoreVMSSEntry struct {
	ResourceGroup ResourceGroup
	VMSS          *compute.VirtualMachineScaleSet
	LastUpdated   time.Time
}

type VMSetStoreVMSS struct {
	// map[ResourceGroup]map[VMSSName]*VMSetStoreVMSSEntry
	// value: sync.Map
	vmssListByResourceGroup *cache.TimedCache
}

func newVMSetStoreVMSS(az *Cloud) (*VMSetStoreVMSS, error) {
	vmssListByResourceGroup, err := cache.NewTimedcache(
		time.Duration(az.Config.VmssCacheTTLInSeconds)*time.Second,
		func(resourceGroup ResourceGroup) (interface{}, error) {
			entry := &sync.Map{}

			scaleSets, err := az.VirtualMachineScaleSetsClient.List(context.Background(), resourceGroup)
			if err != nil {
				klog.Errorf("VirtualMachineScaleSetsClient.List failed: %v", err)
				return nil, err.Error()
			}

			for i := range scaleSets {
				scaleSet := scaleSets[i]
				if to.String(scaleSet.Name) == "" {
					klog.Warning("failed to get the name of VMSS")
					continue
				}
				entry.Store(*scaleSet.Name, &VMSetStoreVMSSEntry{
					VMSS:          &scaleSet,
					ResourceGroup: resourceGroup,
					LastUpdated:   time.Now(),
				})
			}

			return entry, nil
		})
	if err != nil {
		return nil, err
	}

	return &VMSetStoreVMSS{
		vmssListByResourceGroup,
	}, nil
}

func (s *VMSetStoreVMSS) GetVMSSByName(rg ResourceGroup, vn VMSSName, opts ...VMSetStoreOption) (*compute.VirtualMachineScaleSet,
	bool, error) {
	vmssListByName, err := s.vmssListByResourceGroup.Get(rg, defaultStoreOption().Apply(opts).readType)
	if err != nil {
		return nil, false, err
	}

	vmss, found := vmssListByName.(*sync.Map).Load(vn)
	if !found {
		return nil, false, nil
	}

	return vmss.(*VMSetStoreVMSSEntry).VMSS, true, nil
}
