/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	danmv1 "github.com/nokia/danm/crd/apis/danm/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeTenantNetworks implements TenantNetworkInterface
type FakeTenantNetworks struct {
	Fake *FakeDanmV1
	ns   string
}

var tenantnetworksResource = schema.GroupVersionResource{Group: "danm.io", Version: "v1", Resource: "tenantnetworks"}

var tenantnetworksKind = schema.GroupVersionKind{Group: "danm.io", Version: "v1", Kind: "TenantNetwork"}

// Get takes name of the tenantNetwork, and returns the corresponding tenantNetwork object, and an error if there is any.
func (c *FakeTenantNetworks) Get(ctx context.Context, name string, options v1.GetOptions) (result *danmv1.TenantNetwork, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(tenantnetworksResource, c.ns, name), &danmv1.TenantNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*danmv1.TenantNetwork), err
}

// List takes label and field selectors, and returns the list of TenantNetworks that match those selectors.
func (c *FakeTenantNetworks) List(ctx context.Context, opts v1.ListOptions) (result *danmv1.TenantNetworkList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(tenantnetworksResource, tenantnetworksKind, c.ns, opts), &danmv1.TenantNetworkList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &danmv1.TenantNetworkList{ListMeta: obj.(*danmv1.TenantNetworkList).ListMeta}
	for _, item := range obj.(*danmv1.TenantNetworkList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested tenantNetworks.
func (c *FakeTenantNetworks) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(tenantnetworksResource, c.ns, opts))

}

// Create takes the representation of a tenantNetwork and creates it.  Returns the server's representation of the tenantNetwork, and an error, if there is any.
func (c *FakeTenantNetworks) Create(ctx context.Context, tenantNetwork *danmv1.TenantNetwork, opts v1.CreateOptions) (result *danmv1.TenantNetwork, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(tenantnetworksResource, c.ns, tenantNetwork), &danmv1.TenantNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*danmv1.TenantNetwork), err
}

// Update takes the representation of a tenantNetwork and updates it. Returns the server's representation of the tenantNetwork, and an error, if there is any.
func (c *FakeTenantNetworks) Update(ctx context.Context, tenantNetwork *danmv1.TenantNetwork, opts v1.UpdateOptions) (result *danmv1.TenantNetwork, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(tenantnetworksResource, c.ns, tenantNetwork), &danmv1.TenantNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*danmv1.TenantNetwork), err
}

// Delete takes name of the tenantNetwork and deletes it. Returns an error if one occurs.
func (c *FakeTenantNetworks) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(tenantnetworksResource, c.ns, name), &danmv1.TenantNetwork{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeTenantNetworks) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(tenantnetworksResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &danmv1.TenantNetworkList{})
	return err
}

// Patch applies the patch and returns the patched tenantNetwork.
func (c *FakeTenantNetworks) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *danmv1.TenantNetwork, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(tenantnetworksResource, c.ns, name, pt, data, subresources...), &danmv1.TenantNetwork{})

	if obj == nil {
		return nil, err
	}
	return obj.(*danmv1.TenantNetwork), err
}
