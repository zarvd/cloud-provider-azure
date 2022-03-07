package provider

import (
	"fmt"

	"sigs.k8s.io/cloud-provider-azure/pkg/cache"
)

var (
	ErrVMSetStoreCacheMiss = fmt.Errorf("vm set store cache miss")
)

type (
	ResourceGroup = string
	VMSSName      = string
	NodeName      = string
	InstanceID    = string
)

type VMSetStore struct {
	*VMSetStoreVMSS
	*VMSetStoreVMAS
	*VMSetStoreVM
}

func newVMSetStore(az *Cloud) (*VMSetStore, error) {
	vmss, err := newVMSetStoreVMSS(az)
	if err != nil {
		return nil, err
	}

	vmas, err := newVMSetStoreVMAS(az)
	if err != nil {
		return nil, err
	}

	vm, err := newVMSetStoreVM(az)
	if err != nil {
		return nil, err
	}

	return &VMSetStore{
		VMSetStoreVMSS: vmss,
		VMSetStoreVMAS: vmas,
		VMSetStoreVM:   vm,
	}, nil
}

type vmSetStoreOption struct {
	readType cache.AzureCacheReadType
}

func (o vmSetStoreOption) Apply(opts []VMSetStoreOption) vmSetStoreOption {
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

func defaultStoreOption() vmSetStoreOption {
	return vmSetStoreOption{readType: cache.CacheReadTypeDefault}
}

type VMSetStoreOption func(*vmSetStoreOption)

func ForceRefresh() VMSetStoreOption {
	return func(o *vmSetStoreOption) {
		o.readType = cache.CacheReadTypeForceRefresh
	}
}

func UnsafeRead() VMSetStoreOption {
	return func(o *vmSetStoreOption) {
		o.readType = cache.CacheReadTypeUnsafe
	}
}
