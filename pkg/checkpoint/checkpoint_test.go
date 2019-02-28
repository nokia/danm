package checkpoint

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"io/ioutil"
	"testing"

	"github.com/intel/multus-cni/types"
)

const (
	fakeTempFile = "/tmp/kubelet_internal_checkpoint"
)

type fakeCheckpoint struct {
	fileName string
}

func (fc *fakeCheckpoint) WriteToFile(inBytes []byte) error {
	return ioutil.WriteFile(fc.fileName, inBytes, 0600)
}

func (fc *fakeCheckpoint) DeleteFile() error {
	return os.Remove(fc.fileName)
}

func TestCheckpoint(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Checkpoint")
}

var _ = BeforeSuite(func() {
	sampleData := `{
		"Data": {
			"PodDeviceEntries": [
			{
				"PodUID": "970a395d-bb3b-11e8-89df-408d5c537d23",
				"ContainerName": "appcntr1",
				"ResourceName": "intel.com/sriov_net_A",
				"DeviceIDs": [
				"0000:03:02.3",
				"0000:03:02.0"
				],
				"AllocResp": "CikKC3NyaW92X25ldF9BEhogMDAwMDowMzowMi4zIDAwMDA6MDM6MDIuMA=="
			}
			],
			"RegisteredDevices": {
			"intel.com/sriov_net_A": [
				"0000:03:02.1",
				"0000:03:02.2",
				"0000:03:02.3",
				"0000:03:02.0"
			],
			"intel.com/sriov_net_B": [
				"0000:03:06.3",
				"0000:03:06.0",
				"0000:03:06.1",
				"0000:03:06.2"
			]
			}
		},
		"Checksum": 229855270
		}`

	fakeCheckpoint := &fakeCheckpoint{fileName: fakeTempFile}
	err := fakeCheckpoint.WriteToFile([]byte(sampleData))
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("Kubelet checkpoint data read operations", func() {
	Context("Using /tmp/kubelet_internal_checkpoint file", func() {
		var (
			cp            Checkpoint
			err           error
			resourceMap   map[string]*types.ResourceInfo
			resourceInfo  *types.ResourceInfo
			resourceAnnot = "intel.com/sriov_net_A"
		)

		It("should get a Checkpoint instance from file", func() {
			cp, err = getCheckpoint(fakeTempFile)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a ResourceMap instance", func() {
			rmap, err := cp.GetComputeDeviceMap("970a395d-bb3b-11e8-89df-408d5c537d23")
			Expect(err).NotTo(HaveOccurred())
			Expect(rmap).NotTo(BeEmpty())
			resourceMap = rmap
		})

		It("resourceMap should have value for \"intel.com/sriov_net_A\"", func() {
			rInfo, ok := resourceMap[resourceAnnot]
			Expect(ok).To(BeTrue())
			resourceInfo = rInfo
		})

		It("should have 2 deviceIDs", func() {
			Expect(len(resourceInfo.DeviceIDs)).To(BeEquivalentTo(2))
		})

		It("should have \"0000:03:02.3\" in deviceIDs[0]", func() {
			Expect(resourceInfo.DeviceIDs[0]).To(BeEquivalentTo("0000:03:02.3"))
		})

		It("should have \"0000:03:02.0\" in deviceIDs[1]", func() {
			Expect(resourceInfo.DeviceIDs[1]).To(BeEquivalentTo("0000:03:02.0"))
		})
	})
})

var _ = AfterSuite(func() {
	fakeCheckpoint := &fakeCheckpoint{fileName: fakeTempFile}
	err := fakeCheckpoint.DeleteFile()
	Expect(err).NotTo(HaveOccurred())
})
