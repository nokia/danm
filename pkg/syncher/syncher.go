package syncher

import (
  "errors"
  "fmt"
  "strings"
  "sync"
  "time"
  "github.com/containernetworking/cni/pkg/types/current"
)

type cniOpResult struct {
  cniName string
  opResult error
  cniResult *current.Result
}

type Syncher struct {
  expectedNumOfResults int
  cniResults []cniOpResult
  mux sync.Mutex
}

func NewSyncher(numOfResults int) *Syncher {
  syncher := Syncher{}
  syncher.expectedNumOfResults = numOfResults
  return &syncher
}

func (synch *Syncher) PushResult(cniName string, opRes error, cniRes *current.Result) {
  cniOpResult := cniOpResult {
    cniName: cniName,
    opResult: opRes,
    cniResult: cniRes,
  }
  synch.mux.Lock()
  defer synch.mux.Unlock()
  synch.cniResults = append(synch.cniResults, cniOpResult)
}

func (synch *Syncher) GetAggregatedResult() error {
  //Time-out Pod creation if a plugin did not provide result within 10 seconds
  for i := 0; i < 1000; i++ {
    if synch.expectedNumOfResults > len(synch.cniResults) {
      time.Sleep(10 * time.Millisecond)
      continue
    }
    if synch.wasAnyOperationErroneous() {
      return synch.mergeErrorMessages()
    }
    return nil
  }
  return errors.New("CNI operation timed-out after 10 seconds")
}

func (synch *Syncher) wasAnyOperationErroneous() bool {
  for _, cniRes := range synch.cniResults {
    if cniRes.opResult != nil {
      return true
    }
  }
  return false
}

func (synch *Syncher) mergeErrorMessages() error {
  var aggregatedErrors []string
  for _, cniRes := range synch.cniResults {
    if cniRes.opResult != nil {
      aggregatedErrors = append(aggregatedErrors, "CNI operation for network:" + cniRes.cniName + " failed with:" + cniRes.opResult.Error())
    }
  }
  return fmt.Errorf(strings.Join(aggregatedErrors, "\n"))
}

func (synch *Syncher) MergeCniResults() *current.Result {
  aggregatedCniRes := current.Result{}
  for _, cniRes := range synch.cniResults {
    if cniRes.cniResult != nil {
      aggregatedCniRes.Interfaces = append(aggregatedCniRes.Interfaces, cniRes.cniResult.Interfaces...)
      aggregatedCniRes.IPs = append(aggregatedCniRes.IPs, cniRes.cniResult.IPs...)
      aggregatedCniRes.Routes = append(aggregatedCniRes.Routes, cniRes.cniResult.Routes...)
    }
  }
  return &aggregatedCniRes
}

func (synch *Syncher) WasAnyOperationErroneous() bool {
  if len(synch.cniResults) == 0 {
    return false
  }
  return synch.wasAnyOperationErroneous()
}
