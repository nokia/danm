package danm

import (
	"context"
	"errors"
	"strings"

	danmtypes "github.com/danm-cni/danm/crd/apis/danm/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
)

// selectableEpFields mirrors the selectableFields list of the production DanmEp CRD
// Listing DanmEps with a field selector referencing anything else is rejected by the API server, just like it is here
var selectableEpFields = []string{"metadata.name", "metadata.namespace", "spec.reservedIpRefs.v4", "spec.reservedIpRefs.v6"}

type EpClientStub struct {
	TestEps   []danmtypes.DanmEp
	Namespace string
}

func newEpClientStub(eps []danmtypes.DanmEp, ns string) EpClientStub {
	return EpClientStub{TestEps: eps, Namespace: ns}
}

func (epClient EpClientStub) Create(ctx context.Context, obj *danmtypes.DanmEp, options meta_v1.CreateOptions) (*danmtypes.DanmEp, error) {
	return nil, nil
}

func (epClient EpClientStub) Update(ctx context.Context, obj *danmtypes.DanmEp, options meta_v1.UpdateOptions) (*danmtypes.DanmEp, error) {
	return nil, nil
}

func (epClient EpClientStub) Delete(ctx context.Context, name string, options meta_v1.DeleteOptions) error {
	return nil
}

func (epClient EpClientStub) DeleteCollection(ctx context.Context, options meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return nil
}

func (epClient EpClientStub) Get(ctx context.Context, epName string, options meta_v1.GetOptions) (*danmtypes.DanmEp, error) {
	for _, testNet := range epClient.TestEps {
		if testNet.Spec.NetworkName == epName {
			return &testNet, nil
		}
	}
	return nil, nil
}

func (epClient EpClientStub) Watch(ctx context.Context, opts meta_v1.ListOptions) (watch.Interface, error) {
	watch := watch.NewEmptyWatch()
	return watch, nil
}

func (epClient EpClientStub) List(ctx context.Context, opts meta_v1.ListOptions) (*danmtypes.DanmEpList, error) {
	if epClient.TestEps == nil {
		return nil, nil
	}
	for _, ep := range epClient.TestEps {
		if strings.HasPrefix(ep.ObjectMeta.Name, "error") {
			return nil, errors.New("error happened")
		}
	}
	selector, err := parseEpFieldSelector(opts.FieldSelector)
	if err != nil {
		return nil, err
	}
	matchingEps := make([]danmtypes.DanmEp, 0)
	for _, ep := range epClient.TestEps {
		//An empty namespace means a cluster-wide List, just like in the real API
		if epClient.Namespace != "" && ep.ObjectMeta.Namespace != epClient.Namespace {
			continue
		}
		if !selector.Matches(epFieldsOf(ep)) {
			continue
		}
		matchingEps = append(matchingEps, ep)
	}
	epList := danmtypes.DanmEpList{Items: matchingEps}
	return &epList, nil
}

// parseEpFieldSelector emulates how the API server handles a field selector sent for a CRD:
// selecting on a field the CRD does not declare as selectable is answered with a BadRequest instead of a filtered list
func parseEpFieldSelector(rawSelector string) (fields.Selector, error) {
	selector, err := fields.ParseSelector(rawSelector)
	if err != nil {
		return nil, apierrors.NewBadRequest("unable to parse field selector:" + rawSelector)
	}
	for _, requirement := range selector.Requirements() {
		var isSelectable bool
		for _, selectableField := range selectableEpFields {
			if requirement.Field == selectableField {
				isSelectable = true
				break
			}
		}
		if !isSelectable {
			return nil, apierrors.NewBadRequest("field label not supported:" + requirement.Field)
		}
	}
	return selector, nil
}

// epFieldsOf exposes the attributes of a DanmEp the production CRD declares as selectable
func epFieldsOf(ep danmtypes.DanmEp) fields.Set {
	return fields.Set{
		"metadata.name":          ep.ObjectMeta.Name,
		"metadata.namespace":     ep.ObjectMeta.Namespace,
		"spec.reservedIpRefs.v4": ep.Spec.RipRefs.V4,
		"spec.reservedIpRefs.v6": ep.Spec.RipRefs.V6,
	}
}

func (epClient EpClientStub) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts meta_v1.PatchOptions, subresources ...string) (result *danmtypes.DanmEp, err error) {
	return nil, nil
}
