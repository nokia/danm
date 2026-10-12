package danmep

import (
	"testing"

	danmtypes "github.com/danm-cni/danm/crd/apis/danm/v1"
	stubs "github.com/danm-cni/danm/test/stubs/danm"
	"github.com/danm-cni/danm/test/utils"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The DanmEp under deletion is always part of the test set, because DeleteDanmEp only removes it from the API server
// after it decided whether its addresses can be freed. isAddressShared needs to discount this self-match
var sharedIpTestEps = []danmtypes.DanmEp{
	{
		ObjectMeta: meta_v1.ObjectMeta{Name: "epUnderDeletion", Namespace: "rip-test"},
		Spec:       danmtypes.DanmEpSpec{RipRefs: danmtypes.RipReferences{V4: "sharedRip", V6: "sharedRip6"}},
	},
	{
		ObjectMeta: meta_v1.ObjectMeta{Name: "epHoldingTheSameRips", Namespace: "rip-test"},
		Spec:       danmtypes.DanmEpSpec{RipRefs: danmtypes.RipReferences{V4: "sharedRip", V6: "sharedRip6"}},
	},
	{
		ObjectMeta: meta_v1.ObjectMeta{Name: "epHoldingAnotherRip", Namespace: "rip-test"},
		Spec:       danmtypes.DanmEpSpec{RipRefs: danmtypes.RipReferences{V4: "otherRip", V6: "otherRip6"}},
	},
	{
		ObjectMeta: meta_v1.ObjectMeta{Name: "epWithoutAnyRip", Namespace: "rip-test"},
		Spec:       danmtypes.DanmEpSpec{},
	},
	{
		ObjectMeta: meta_v1.ObjectMeta{Name: "epInAnotherNamespace", Namespace: "another-tenant"},
		Spec:       danmtypes.DanmEpSpec{RipRefs: danmtypes.RipReferences{V4: "lonelyRip", V6: "lonelyRip6"}},
	},
	{
		ObjectMeta: meta_v1.ObjectMeta{Name: "epHoldingTheLonelyRipInAnotherNamespace", Namespace: "yet-another-tenant"},
		Spec:       danmtypes.DanmEpSpec{RipRefs: danmtypes.RipReferences{V4: "lonelyRip", V6: "lonelyRip6"}},
	},
}

// A DanmEp named with the "error" prefix makes the stubbed List fail with a generic, non-API error
// isAddressShared discards every List error, so a failed query currently reads as "not shared", and the address is freed
var listErrorTestEps = []danmtypes.DanmEp{
	{
		ObjectMeta: meta_v1.ObjectMeta{Name: "errorEp", Namespace: "rip-test"},
		Spec:       danmtypes.DanmEpSpec{RipRefs: danmtypes.RipReferences{V4: "sharedRip"}},
	},
}

var isAddressSharedTcs = []struct {
	tcName         string
	testEps        []danmtypes.DanmEp
	fieldSelector  string
	ns             string
	ripName        string
	isSharedResult bool
}{
	{"onlyTheEpUnderDeletionHoldsTheRip", sharedIpTestEps, V4RipFieldSelector, "rip-test", "otherRip", false},
	{"anotherEpHoldsTheSameRip", sharedIpTestEps, V4RipFieldSelector, "rip-test", "sharedRip", true},
	{"onlyTheEpUnderDeletionHoldsTheRipV6", sharedIpTestEps, V6RipFieldSelector, "rip-test", "otherRip6", false},
	{"anotherEpHoldsTheSameRipV6", sharedIpTestEps, V6RipFieldSelector, "rip-test", "sharedRip6", true},
	{"ripIsOnlySharedInAnotherNamespace", sharedIpTestEps, V4RipFieldSelector, "another-tenant", "lonelyRip", false},
	{"ripIsOnlySharedInAnotherNamespaceV6", sharedIpTestEps, V6RipFieldSelector, "another-tenant", "lonelyRip6", false},
	{"noEpReferencesTheRip", sharedIpTestEps, V4RipFieldSelector, "rip-test", "unreferencedRip", false},
	{"clusterWithoutAnyDanmEp", nil, V4RipFieldSelector, "rip-test", "sharedRip", false},
	{"crdDoesNotDeclareTheFieldSelectable", sharedIpTestEps, "spec.reservedIpRefs.v5", "rip-test", "sharedRip", false},
	{"listFailsWithANonApiError", listErrorTestEps, V4RipFieldSelector, "rip-test", "sharedRip", false},
}

func TestIsAddressShared(t *testing.T) {
	for _, tc := range isAddressSharedTcs {
		t.Run(tc.tcName, func(t *testing.T) {
			testArtifacts := utils.TestArtifacts{TestEps: tc.testEps}
			epClientStub := stubs.NewClientSetStub(testArtifacts)
			isShared := isAddressShared(epClientStub, tc.fieldSelector, tc.ns, tc.ripName)
			if isShared != tc.isSharedResult {
				t.Errorf("Sharing verdict:%v does not match with expectation:%v", isShared, tc.isSharedResult)
			}
		})
	}
}
