package provider

import (
	"sync"
	"time"

	"k8s.io/klog/v2"

	"sigs.k8s.io/cloud-provider-azure/pkg/cache"
	"sigs.k8s.io/cloud-provider-azure/pkg/provider/virtualmachine"
)

type (
	VMSetStoreVMEntry struct {
		VM          *virtualmachine.VirtualMachine
		LastUpdated time.Time
	}

	VMSetStoreVM struct {
		// Source of truth for VM entries.
		// map[ResourceGroup][]*VMSetStoreVMEntry
		vmListByResourceGroup *cache.TimedCache

		// Index of VM entries by name.
		// map[VMName]*VMSetStoreVMEntry
		vmByName *cache.TimedCache
	}
)

func newVMSetStoreVM(az *Cloud) (*VMSetStoreVM, error) {
	vmListByResourceGroup, err := cache.NewTimedcache(
		time.Duration(az.Config.VMCacheTTLInSeconds)*time.Second,
		func(rg ResourceGroup) (interface{}, error) {
			// Currently InstanceView request are used by azure_zones, while the calls come after non-InstanceView
			// request. If we first send an InstanceView request and then a non InstanceView request, the second
			// request will still hit throttling. This is what happens now for cloud controller manager: In this
			// case we do get instance view every time to fulfill the azure_zones requirement without hitting
			// throttling.
			// Consider adding separate parameter for controlling 'InstanceView' once node update issue #56276 is fixed
			ctx, cancel := getContextWithCancel()
			defer cancel()

			vms, verr := az.VirtualMachinesClient.List(ctx, rg)
			exists, rerr := checkResourceExistsFromError(verr)
			if rerr != nil {
				return nil, rerr.Error()
			}

			if !exists {
				klog.V(2).Infof("Virtual machine %q not fetch")
				return nil, nil
			}

			var entries []*VMSetStoreVMEntry
			for _, vm := range vms {
				entries = append(entries, &VMSetStoreVMEntry{
					VM:          virtualmachine.FromVirtualMachine(rg, &vm),
					LastUpdated: time.Now(),
				})
			}

			// TODO(lodrem): List VMSSVM

			return entries, nil
		},
	)

	if err != nil {
		return nil, err
	}

	vmByName, err := cache.NewTimedcache(
		time.Duration(az.Config.VMCacheTTLInSeconds)*time.Second,
		func(name NodeName) (interface{}, error) {

			resourceGroup, err := az.GetNodeResourceGroup(name)
			if err != nil {
				return nil, err
			}

			v, err := vmListByResourceGroup.Get(resourceGroup, cache.CacheReadTypeDefault)
			if err != nil {
				return nil, err
			}

			for _, entry := range v.([]*VMSetStoreVMEntry) {
				if entry.VM.Name == name {
					return entry.VM, nil
				}
			}

			return nil, ErrVMSetStoreCacheMiss
		},
	)
	if err != nil {
		return nil, err
	}

	return &VMSetStoreVM{
		vmListByResourceGroup,
		vmByName,
	}, nil

}

// GetVMByName returns the VM for the given node name.
// It contains the VM and VMSSVM information.
func (s *VMSetStoreVM) GetVMByName(name NodeName, opts ...VMSetStoreOption) (*virtualmachine.VirtualMachine, error) {
	vm, err := s.vmByName.Get(name, defaultStoreOption().Apply(opts).readType)
	if err != nil {
		return nil, err
	}

	return vm.(*virtualmachine.VirtualMachine), nil
}

// ListVMByVMSS returns the VMs for the given VMSS.
func (s *VMSetStoreVM) ListVMByVMSS(rg ResourceGroup, vn VMSSName, opts ...VMSetStoreOption) ([]*virtualmachine.VirtualMachine, error) {
	vmListByVMSSName, err := s.vmListByResourceGroup.Get(rg, defaultStoreOption().Apply(opts).readType)
	if err != nil {
		return nil, err
	}

	vmList, found := vmListByVMSSName.(*sync.Map).Load(vn)
	if !found {
		return nil, ErrVMSetStoreCacheMiss
	}

	var vms []*virtualmachine.VirtualMachine

	vmList.(*sync.Map).Range(func(key, value interface{}) bool {
		vms = append(vms, value.(*VMSetStoreVMEntry).VM)
		return true
	})

	return vms, nil
}

// GetVMByVMSSAndInstanceID returns the VM for the given VMSS and instanceID.
func (s *VMSetStoreVM) GetVMByVMSSAndInstanceID(rg ResourceGroup, vn VMSSName, instanceID InstanceID, opts ...VMSetStoreOption) (*virtualmachine.VirtualMachine, error) {
	vmListByVMSSName, err := s.vmListByResourceGroup.Get(rg, defaultStoreOption().Apply(opts).readType)
	if err != nil {
		return nil, err
	}

	vmList, found := vmListByVMSSName.(*sync.Map).Load(vn)
	if !found {
		return nil, ErrVMSetStoreCacheMiss
	}

	var vm *virtualmachine.VirtualMachine

	vmList.(*sync.Map).Range(func(key, value interface{}) bool {
		v := value.(*virtualmachine.VirtualMachine)

		if v.IsVirtualMachineScaleSetVM() && v.InstanceID == instanceID {
			vm = v
			return false
		}
		return true
	})

	return vm, nil
}

func (s *VMSetStoreVM) GetVMByVMSSAndName(rg ResourceGroup, vn VMSSName, name NodeName, opts ...VMSetStoreOption) (*virtualmachine.VirtualMachine, error) {
	vmListByVMSSName, err := s.vmListByResourceGroup.Get(rg, defaultStoreOption().Apply(opts).readType)
	if err != nil {
		return nil, err
	}

	vmList, found := vmListByVMSSName.(*sync.Map).Load(vn)
	if !found {
		return nil, ErrVMSetStoreCacheMiss
	}

	var vm *virtualmachine.VirtualMachine

	vmList.(*sync.Map).Range(func(key, value interface{}) bool {
		v := value.(*virtualmachine.VirtualMachine)

		if v.Name == name {
			vm = v
			return false
		}
		return true
	})

	return vm, nil
}
