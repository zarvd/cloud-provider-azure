package provider

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-07-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	azcache "sigs.k8s.io/cloud-provider-azure/pkg/cache"
	"sigs.k8s.io/cloud-provider-azure/pkg/consts"
	"sigs.k8s.io/cloud-provider-azure/pkg/provider/node"
	"sigs.k8s.io/cloud-provider-azure/pkg/provider/virtualmachine"
)

type ScaleSet struct {
	enabled struct {
		vmas         bool
		vmssUniform  bool
		vmssFlexible bool
	}

	az *Cloud

	vmas     *availabilitySet
	uniform  *uniformScaleSet
	flexible *flexibleScaleSet

	store *VMSetStore
}

func newScaleSet(az *Cloud, vmas, vmssUniform, vmssFlexible bool) (*ScaleSet, error) {

	scaleSet := &ScaleSet{
		az: az,

		enabled: struct {
			vmas         bool
			vmssUniform  bool
			vmssFlexible bool
		}{
			vmas,
			vmssUniform,
			vmssFlexible,
		},
	}

	var err error

	scaleSet.uniform, err = newUniformScaleSet(az)
	if err != nil {
		return nil, err
	}

	scaleSet.flexible, err = newFlexibleScaleSet()
	if err != nil {
		return nil, err
	}

	scaleSet.vmas, err = newAvailabilitySet(az)
	if err != nil {
		return nil, err
	}

	scaleSet.store, err = newVMSetStore(az)
	if err != nil {
		return nil, err
	}

	return scaleSet, nil
}

func (s *ScaleSet) GetInstanceIDByNodeName(name string) (string, error) {
	panic("implement me")
}

func (s *ScaleSet) GetInstanceTypeByNodeName(name string) (string, error) {
	vm, err := s.store.GetVMByName(name, UnsafeRead())
	if err != nil {
		return "", err
	}

	if vm.ManagedByVMSS() {
		if vm.SKU != nil {
			return to.String(vm.SKU.Name), nil
		}
	} else {
		if vm.HardwareProfile != nil {
			return string(vm.HardwareProfile.VMSize), nil
		}
	}
	return "", nil
}

func (s *ScaleSet) GetIPByNodeName(name string) (string, string, error) {
	//TODO implement me
}

func (s *ScaleSet) GetPrimaryInterface(nodeName string) (network.Interface, error) {
	panic("implement me")
}

func (s *ScaleSet) GetNodeNameByProviderID(providerID string) (types.NodeName, error) {
	// FIXME(lodrem): be compatible with the providerID provided by the disk which only contains instance ID.

	p, err := node.ParseProviderID(providerID)
	if err != nil {
		return "", err
	}

	if p.ManagedByVMAS() {
		return types.NodeName(p.VMName), nil
	}

	// provider must be VMSS

	var vm *virtualmachine.VirtualMachine

	if p.VMSSOrchestration().IsUniform() {
		instanceID := p.UniformVMSSInstanceID()
		vm, err = s.store.GetVMByVMSSAndInstanceID(p.ResourceGroup, p.VMSSName, instanceID, UnsafeRead())
	} else {
		vm, err = s.store.GetVMByVMSSAndName(p.ResourceGroup, p.VMSSName, p.VMName, UnsafeRead())
	}

	if err != nil {
		klog.Errorf("Unable to find node by providerID %s: %v", providerID, err)
		return "", err
	}

	if vm.OSProfile != nil && vm.OSProfile.ComputerName != nil {
		nodeName := strings.ToLower(*vm.OSProfile.ComputerName)
		return types.NodeName(nodeName), nil
	}

	return "", nil
}

func (s *ScaleSet) GetZoneByNodeName(name string) (cloudprovider.Zone, error) {
	//TODO implement me
	panic("implement me")
}

func (s *ScaleSet) GetPrimaryVMSetName() string {
	//TODO implement me
	panic("implement me")
}

func (s *ScaleSet) GetVMSetNames(svc *v1.Service, nodes []*v1.Node) (*[]string, error) {
	lbMode, isAuto, svcAvailabilitySetNames := s.az.getServiceLoadBalancerMode(svc)
	isSingleLB := s.az.useStandardLoadBalancer() && !s.az.EnableMultipleStandardLoadBalancers

	if !lbMode || isSingleLB {
		return &[]string{s.az.Config.PrimaryScaleSetName}, nil
	}

	for _, node := range nodes {
		vm, err := s.store.GetVMByName(node.Name)
		if err != nil {
			return nil, err
		}
	}
}

func (s *ScaleSet) GetNodeVMSetName(node *v1.Node) (string, error) {
	vm, err := s.store.VMSetStoreVM.GetVMByName(node.Name)
	if err != nil {
		return "", err
	}

	if vm.ManagedByVMSS() {
		return vm.VMSSName, nil
	} else {
		return vm.VMASName, nil
	}
}

func (s *ScaleSet) EnsureHostsInPool(service *v1.Service, nodes []*v1.Node, backendPoolID string, vmSetName string) error {
	//TODO implement me
	panic("implement me")
}

func (s *ScaleSet) EnsureHostInPool(service *v1.Service, nodeName types.NodeName, backendPoolID string, vmSetName string) (string, *virtualmachine.VirtualMachine, error) {
	//TODO implement me
	panic("implement me")
}

func (s *ScaleSet) EnsureBackendPoolDeleted(service *v1.Service, backendPoolID, vmSetName string, backendAddressPools *[]network.BackendAddressPool, deleteFromVMSet bool) error {
	//TODO implement me
	panic("implement me")
}

func (s *ScaleSet) EnsureBackendPoolDeletedFromVMSets(vmSetNamesMap map[string]bool, backendPoolID string) error {
	//TODO implement me
	panic("implement me")
}

func (s *ScaleSet) AttachDisk(ctx context.Context, nodeName types.NodeName, diskMap map[string]*AttachDiskOptions) (*azure.Future, error) {
	//TODO implement me
	panic("implement me")
}

func (s *ScaleSet) DetachDisk(ctx context.Context, nodeName types.NodeName, diskMap map[string]string) error {
	//TODO implement me
	panic("implement me")
}

func (s *ScaleSet) WaitForUpdateResult(ctx context.Context, future *azure.Future, resourceGroupName, source string) error {
	//TODO implement me
	panic("implement me")
}

func (s *ScaleSet) GetDataDisks(nodeName types.NodeName, crt azcache.AzureCacheReadType) ([]compute.DataDisk, *string, error) {
	//TODO implement me
	panic("implement me")
}

func (s *ScaleSet) UpdateVM(ctx context.Context, nodeName types.NodeName) error {
	//TODO implement me
	panic("implement me")
}

func (s *ScaleSet) GetPowerStatusByNodeName(name string) (string, error) {
	vm, err := s.store.GetVMByName(name, UnsafeRead())
	if err != nil {
		return "", err
	}

	for _, status := range *vm.InstanceViewStatus {
		code := to.String(status.Code)
		if strings.HasPrefix(code, vmPowerStatePrefix) {
			return strings.TrimPrefix(code, vmPowerStatePrefix), nil
		}
	}

	return vmPowerStateStopped, nil
}

func (s *ScaleSet) GetProvisioningStateByNodeName(name string) (string, error) {
	vm, err := s.store.GetVMByName(name)
	if err != nil {
		return "", err
	}

	return vm.ProvisioningState, nil
}

func (s *ScaleSet) GetPrivateIPsByNodeName(name string) ([]string, error) {
	//TODO implement me
	panic("implement me")
}

func (s *ScaleSet) GetNodeNameByIPConfigurationID(ipConfigurationID string) (string, string, error) {
	//TODO implement me
	panic("implement me")
}

func (s *ScaleSet) GetNodeCIDRMasksByProviderID(providerID string) (int, int, error) {
	p, err := node.ParseProviderID(providerID)
	if err != nil {
		return 0, 0, err
	}

	var tags map[string]*string
	if p.ManagedByVMSS() {
		// TODO(lodrem): refactor VMSS cache managed by `ScaleSet` instead of `uniform`
		vmss, err := s.uniform.getVMSS(p.VMSSName, azcache.CacheReadTypeDefault)
		if err != nil {
			return 0, 0, err
		}
		tags = vmss.Tags
	} else {
		// TODO(lodrem): refactor VMSS cache managed by `ScaleSet` instead of `vmas`
		vmas, err := s.vmas.getAvailabilitySetByNodeName(p.VMName, azcache.CacheReadTypeDefault)
		if err != nil {
			if errors.Is(err, cloudprovider.InstanceNotFound) {
				return consts.DefaultNodeMaskCIDRIPv4, consts.DefaultNodeMaskCIDRIPv6, nil
			}
			return 0, 0, err
		}

		tags = vmas.Tags
	}

	var ipv4Mask, ipv6Mask int
	if v4, ok := tags[consts.VMSetCIDRIPV4TagKey]; ok && v4 != nil {
		ipv4Mask, err = strconv.Atoi(to.String(v4))
		if err != nil {
			klog.Errorf("GetNodeCIDRMasksByProviderID: error when paring the value of the ipv4 mask size %s: %v", to.String(v4), err)
		}
	}
	if v6, ok := tags[consts.VMSetCIDRIPV6TagKey]; ok && v6 != nil {
		ipv6Mask, err = strconv.Atoi(to.String(v6))
		if err != nil {
			klog.Errorf("GetNodeCIDRMasksByProviderID: error when paring the value of the ipv6 mask size%s: %v", to.String(v6), err)
		}
	}

	return ipv4Mask, ipv6Mask, nil
}

func (s *ScaleSet) GetAgentPoolVMSetNames(nodes []*v1.Node) (*[]string, error) {

	var names []string

	for _, n := range nodes {
		vm, err := s.store.VMSetStoreVM.GetVMByName(n.Name)
		if err != nil {
			return nil, err
		}
		if vm.ManagedByVMSS() {
			names = append(names, vm.VMSSName)
		} else {
			names = append(names, vm.VMASName)
		}
	}

	return &names, nil
}
