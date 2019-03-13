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
  CniName string
  OpResult error
  CniResult *current.Result
}

type Syncher struct {
  ExpectedNumOfResults int
  CniResults []cniOpResult
  mux sync.Mutex
}

func NewSyncher(numOfResults int) *Syncher {
  syncher := Syncher{}
  syncher.ExpectedNumOfResults = numOfResults
  return &syncher
}

func (synch *Syncher) PushResult(cniName string, opRes error, cniRes *current.Result) {
  cniOpResult := cniOpResult {
    CniName: cniName,
    OpResult: opRes,
    CniResult: cniRes,
  }
  synch.mux.Lock()
  defer synch.mux.Unlock()
  synch.CniResults = append(synch.CniResults, cniOpResult)
}

func (synch *Syncher) GetAggregatedResult() error {
  //Time-out Pod creation if a plugin did not provide result within 10 seconds
  for i := 0; i < 1000; i++ {
    if synch.ExpectedNumOfResults > len(synch.CniResults) {
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
  for _, cniRes := range synch.CniResults {
    if cniRes.OpResult != nil {
      return true
    }
  }
  return false
}

func (synch *Syncher) mergeErrorMessages() error {
  var aggregatedErrors []string
  for _, cniRes := range synch.CniResults {
    if cniRes.OpResult != nil {
      aggregatedErrors = append(aggregatedErrors, "CNI operation for network:" + cniRes.CniName + " failed with:" + cniRes.OpResult.Error())
    }
  }
  return fmt.Errorf(strings.Join(aggregatedErrors, "\n"))
}

func (synch *Syncher) MergeCniResults() *current.Result {
  aggregatedCniRes := current.Result{}
  for _, cniRes := range synch.CniResults {
    if cniRes.CniResult != nil {
      aggregatedCniRes.Interfaces = append(aggregatedCniRes.Interfaces, cniRes.CniResult.Interfaces...)
      aggregatedCniRes.IPs = append(aggregatedCniRes.IPs, cniRes.CniResult.IPs...)
      aggregatedCniRes.Routes = append(aggregatedCniRes.Routes, cniRes.CniResult.Routes...)
    }
  }
  return &aggregatedCniRes
}

func (synch *Syncher) WasAnyOperationErroneous() bool {
  if len(synch.CniResults) == 0 {
    return false
  }
  return synch.wasAnyOperationErroneous()
}
