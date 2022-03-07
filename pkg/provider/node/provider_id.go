package node

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"sigs.k8s.io/cloud-provider-azure/pkg/provider/virtualmachine"
)

var (
	vmRegexp = regexp.MustCompile(
		`^azure:///subscriptions/([^/]+)/resourceGroups/([^/]+)/providers/Microsoft.Compute/virtualMachines/(.+)$`)
	vmssRegexp = regexp.MustCompile(
		`^azure:///subscriptions/([^/]+)/resourceGroups/([^/]+)/providers/Microsoft.Compute/virtualMachineScaleSets/([^/]+)/virtualMachines/([^/]+)$`)
)

type ProviderID struct {
	VMManage virtualmachine.Manage

	SubscriptionID string
	ResourceGroup  string

	VMSSName string
	VMName   string
}

func ParseProviderID(id string) (ProviderID, error) {
	res := vmRegexp.FindStringSubmatch(id)
	if len(res) == 4 {
		return ProviderID{
			VMManage: virtualmachine.VMAS,

			SubscriptionID: res[1],
			ResourceGroup:  res[2],

			VMName: res[3],
		}, nil
	}

	res = vmssRegexp.FindStringSubmatch(id)
	if len(res) == 5 {
		return ProviderID{
			VMManage: virtualmachine.VMSS,

			SubscriptionID: res[1],
			ResourceGroup:  res[2],

			VMSSName: res[3],
			VMName:   res[4],
		}, nil
	}

	return ProviderID{}, errors.New("invalid provider id")
}

func (p ProviderID) UniformVMSSInstanceID() string {
	if !p.VMSSOrchestration().IsUniform() {
		panic("should never call this function for non-uniform vmss")
	}

	parts := strings.Split(p.VMName, "_")

	if len(parts) < 2 {
		panic("invalid vmss instance id")
	}

	return parts[len(parts)-1]
}

func (p ProviderID) ManagedByVMAS() bool {
	return p.VMManage == virtualmachine.VMAS
}

func (p ProviderID) ManagedByVMSS() bool {
	return p.VMManage == virtualmachine.VMSS
}

func (p ProviderID) VMSSOrchestration() virtualmachine.VMSSMode {
	if !p.ManagedByVMSS() {
		panic("should never call this function on non-VMSS provider")
	}
	if strings.HasPrefix(p.VMName, p.VMSSName) {
		return virtualmachine.Uniform
	} else {
		return virtualmachine.Flexible
	}
}

func (p ProviderID) String() string {
	if p.ManagedByVMAS() {
		return fmt.Sprintf("azure:///subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachines/%s",
			p.SubscriptionID, p.ResourceGroup, p.VMName)
	} else {
		return fmt.Sprintf("azure:///subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachineScaleSets/%s/virtualMachines/%s",
			p.SubscriptionID, p.ResourceGroup, p.VMSSName, p.VMName)
	}
}
