//go:generate bash -c "cg(){ go list -m -f {{.Dir}} k8s.io/code-generator;}; crd(){ cat<<<'github.com/nokia/danm/crd';}; GOFLAGS='' bash $(cg)/generate-groups.sh all $(crd)/client $(crd)/apis danm:v1 --go-header-file $(cg)/hack/boilerplate.go.txt"
// +k8s:deepcopy-gen=package

// Package v1 is the v1 version of the API.
// +groupName=danm.k8s.io
package v1
