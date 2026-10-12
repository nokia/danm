package webhook

import (
	"encoding/json"
	"errors"
	"log"
	"strconv"

	"github.com/danm-cni/danm/pkg/admit"
	httpstub "github.com/danm-cni/danm/test/stubs/http"
	"k8s.io/api/admission/v1beta1"
)

func ValidateHttpResponse(writer *httpstub.ResponseWriterStub, isErrorExpected bool, expectedPatches []admit.Patch) error {
	if writer.RespHeader.Get("Content-Type") != "application/json" {
		return errors.New("Content-Type is not set to application/json in the HTTP Header")
	}
	response, err := writer.GetAdmissionResponse()
	if err != nil {
		return err
	}
	if isErrorExpected {
		if response.Allowed {
			return errors.New("request would have been admitted but we expected an error")
		}
		if response.Result.Message == "" {
			return errors.New("a faulty response was sent without explanation")
		}
	} else {
		if !response.Allowed {
			return errors.New("request would have been denied with: " + response.Result.Message + " but we expected it to pass through validation")
		}
		if response.Result != nil {
			return errors.New("an unnecessary Result message is put into a successful response")
		}
	}
	return validatePatches(response, expectedPatches)
}

func validatePatches(response *v1beta1.AdmissionResponse, expectedPatches []admit.Patch) error {
	if len(expectedPatches) == 0 {
		if response.Patch != nil {
			return errors.New("did not expect any patches but some were included in the admission response")
		}
		return nil
	}
	var patches []admit.Patch
	err := json.Unmarshal(response.Patch, &patches)
	if err != nil {
		return err
	}
	for _, recPatch := range patches {
		log.Println("Received patch for path:" + recPatch.Path)
	}
	for _, expPatch := range expectedPatches {
		var foundMatchingPatch bool
		for _, recPatch := range patches {
			if expPatch.Path == recPatch.Path {
				foundMatchingPatch = true
				if expPatch.Op != "" && expPatch.Op != recPatch.Op {
					return errors.New("Patch modifying path:" + expPatch.Path + " used operation:" + recPatch.Op + " instead of the expected:" + expPatch.Op)
				}
				if expPatch.Value != nil {
					err = matchPatchValues(expPatch, recPatch)
					if err != nil {
						return err
					}
				}
				break
			}
		}
		if !foundMatchingPatch {
			return errors.New("Patch expected to modify path:" + expPatch.Path + " was not included in the response")
		}
	}
	if len(patches) != len(expectedPatches) {
		return errors.New("received number of patches:" + strconv.Itoa(len(patches)) + " was not what we expected:" + strconv.Itoa(len(expectedPatches)))
	}

	return nil
}

func matchPatchValues(expPatch, recPatch admit.Patch) error {
	expValue, err := json.Marshal(expPatch.Value)
	if err != nil {
		return err
	}
	recValue, err := json.Marshal(recPatch.Value)
	if err != nil {
		return err
	}
	if string(expValue) != string(recValue) {
		return errors.New("Patch modifying path:" + expPatch.Path + " contained value:" + string(recValue) + " instead of the expected:" + string(expValue))
	}
	return nil
}
