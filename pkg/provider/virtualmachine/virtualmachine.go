/*
Copyright 2022 The Kubernetes Authors.

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

package virtualmachine

import (
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-07-01/compute"
	"github.com/Azure/go-autorest/autorest/to"
)

type Variant string

const (
	VariantVirtualMachine           Variant = "VirtualMachine"
	VariantVirtualMachineScaleSetVM Variant = "VirtualMachineScaleSetVM"
)

type Manage string

const (
	VMSS Manage = "vmss"
	VMAS Manage = "vmas"
)

type VMSSMode string

const (
	Uniform  VMSSMode = "uniform"
	Flexible VMSSMode = "flexible"
)

func (o VMSSMode) IsUniform() bool {
	return o == Uniform
}

func (o VMSSMode) IsFlexible() bool {
	return o == Flexible
}

type ManageOption = func(*VirtualMachine)

// ByVMSS specifies that the virtual machine is managed by a virtual machine scale set.
func ByVMSS(vmssName string) ManageOption {
	return func(vm *VirtualMachine) {
		vm.Manage = VMSS
		vm.VMSSName = vmssName
	}
}

func ByVMAS(vmasName string) ManageOption {
	return func(vm *VirtualMachine) {
		vm.Manage = VMAS
		vm.VMSSName = vmasName
	}
}

type VirtualMachine struct {
	Variant Variant
	vm      *compute.VirtualMachine
	vmssVM  *compute.VirtualMachineScaleSetVM

	ResourceGroup string
	Manage        Manage
	VMSSName      string
	VMASName      string

	// re-export fields
	// common fields
	ID                 string
	Name               string
	Location           string
	Tags               map[string]string
	Zones              []string
	Type               string
	Plan               *compute.Plan
	Resources          *[]compute.VirtualMachineExtension
	OSProfile          *compute.OSProfile
	NetworkProfile     *compute.NetworkProfile
	HardwareProfile    *compute.HardwareProfile
	InstanceViewStatus *[]compute.InstanceViewStatus
	ProvisioningState  string

	// fields of VirtualMachine
	Identity                 *compute.VirtualMachineIdentity
	VirtualMachineProperties *compute.VirtualMachineProperties

	// fields of VirtualMachineScaleSetVM
	InstanceID                         string
	SKU                                *compute.Sku
	VirtualMachineScaleSetVMProperties *compute.VirtualMachineScaleSetVMProperties
}

func FromVirtualMachine(resourceGroup string, vm *compute.VirtualMachine, opts ...ManageOption) *VirtualMachine {
	v := &VirtualMachine{
		vm:            vm,
		Variant:       VariantVirtualMachine,
		Manage:        VMAS,
		ResourceGroup: resourceGroup,

		ID:                 to.String(vm.ID),
		Name:               to.String(vm.Name),
		Type:               to.String(vm.Type),
		Location:           to.String(vm.Location),
		Tags:               to.StringMap(vm.Tags),
		Zones:              to.StringSlice(vm.Zones),
		Plan:               vm.Plan,
		Resources:          vm.Resources,
		OSProfile:          vm.OsProfile,
		NetworkProfile:     vm.NetworkProfile,
		HardwareProfile:    vm.HardwareProfile,
		InstanceViewStatus: vm.InstanceView.Statuses, // FIXME: check pointer
		ProvisioningState:  to.String(vm.ProvisioningState),

		Identity:                 vm.Identity,
		VirtualMachineProperties: vm.VirtualMachineProperties,
	}

	if vm.VirtualMachineProperties != nil &&
		vm.VirtualMachineProperties.VirtualMachineScaleSet != nil &&
		vm.VirtualMachineProperties.VirtualMachineScaleSet.ID != nil {
		// managed by VMSS
		parts := strings.Split(*vm.VirtualMachineProperties.VirtualMachineScaleSet.ID, "/")
		vmssName := parts[len(parts)-1]
		opts = append(opts, ByVMSS(vmssName))
	}

	if vm.VirtualMachineProperties != nil &&
		vm.VirtualMachineProperties.AvailabilitySet != nil &&
		vm.VirtualMachineProperties.AvailabilitySet.ID != nil {
		// managed by VMSS
		parts := strings.Split(*vm.VirtualMachineProperties.AvailabilitySet.ID, "/")
		vmasName := parts[len(parts)-1]
		opts = append(opts, ByVMAS(vmasName))
	}

	for _, opt := range opts {
		opt(v)
	}

	return v
}

func FromVirtualMachineScaleSetVM(resourceGroup string, vm *compute.VirtualMachineScaleSetVM, opt ManageOption) *VirtualMachine {
	v := &VirtualMachine{
		Variant:       VariantVirtualMachineScaleSetVM,
		Manage:        VMSS,
		ResourceGroup: resourceGroup,
		vmssVM:        vm,

		ID:                 to.String(vm.ID),
		Name:               to.String(vm.Name),
		Type:               to.String(vm.Type),
		Location:           to.String(vm.Location),
		Tags:               to.StringMap(vm.Tags),
		Zones:              to.StringSlice(vm.Zones),
		Plan:               vm.Plan,
		Resources:          vm.Resources,
		OSProfile:          vm.OsProfile,
		NetworkProfile:     vm.NetworkProfile,
		HardwareProfile:    vm.HardwareProfile,
		InstanceViewStatus: vm.InstanceView.Statuses,
		ProvisioningState:  to.String(vm.ProvisioningState),

		SKU:                                vm.Sku,
		InstanceID:                         to.String(vm.InstanceID),
		VirtualMachineScaleSetVMProperties: vm.VirtualMachineScaleSetVMProperties,
	}

	// TODO: should validate manage option
	// VirtualMachineScaleSetVM should always be managed by VMSS
	opt(v)

	return v
}

func (vm *VirtualMachine) IsVirtualMachine() bool {
	return vm.Variant == VariantVirtualMachine
}

func (vm *VirtualMachine) IsVirtualMachineScaleSetVM() bool {
	return vm.Variant == VariantVirtualMachineScaleSetVM
}

func (vm *VirtualMachine) ManagedByVMSS() bool {
	return vm.Manage == VMSS
}

func (vm *VirtualMachine) AsVirtualMachine() *compute.VirtualMachine {
	return vm.vm
}

func (vm *VirtualMachine) AsVirtualMachineScaleSetVM() *compute.VirtualMachineScaleSetVM {
	return vm.vmssVM
}
