package node

import (
	"fmt"
	"reflect"
	"testing"
)

func TestParseProviderID(t *testing.T) {
	type args struct {
		id string
	}
	tests := []struct {
		args    args
		want    ProviderID
		wantErr bool
	}{
		{args: args{id: ""}, want: ProviderID{}, wantErr: true},
		{args: args{id: "azure:///"}, want: ProviderID{}, wantErr: true},
		{args: args{
			id: "azure:///subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/foo_1-2/providers/Microsoft.Compute/virtualMachines/flex-01-control-plane-tq6k5"},
			want: ProviderID{
				SubscriptionID:   "8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8",
				ResourceGroup:    "foo_1-2",
				VMName:           "flex-01-control-plane-tq6k5",
				ResourceProvider: RPVirtualMachine,
			},
			wantErr: false,
		},
		{args: args{
			id: "azure:///subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/foo_2-4/providers/Microsoft.Compute/virtualMachineScaleSets/flex-01-mp-0/virtualMachines/1b7d753e"},
			want: ProviderID{
				SubscriptionID:   "8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8",
				ResourceGroup:    "foo_2-4",
				VMSSName:         "flex-01-mp-0",
				VMName:           "1b7d753e",
				ResourceProvider: RPVirtualMachineScaleSets,
			},
			wantErr: false,
		},
	}
	for idx, tt := range tests {
		t.Run(fmt.Sprintf("test #%d", idx), func(t *testing.T) {
			got, err := ParseProviderID(tt.args.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseProviderID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseProviderID() got = %v, want %v", got, tt.want)
			}
		})
	}
}
