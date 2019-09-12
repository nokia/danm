package syncher

import (
  "errors"
  "fmt"
  "strconv"
  "strings"
  "sync"
  "time"
  "github.com/containernetworking/cni/pkg/types/current"
)

const (
  MaximumAllowedTime = 3000
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
  synch.mux.Lock()
  defer synch.mux.Unlock()
  cniOpResult := cniOpResult {
    CniName: cniName,
    OpResult: opRes,
    CniResult: cniRes,
  }
  synch.CniResults = append(synch.CniResults, cniOpResult)
}

func (synch *Syncher) GetAggregatedResult() error {
  //Time-out Pod creation if plugins did not provide results within the configured timeframe
  for i := 0; i < MaximumAllowedTime; i++ {
    if synch.ExpectedNumOfResults > len(synch.CniResults) {
      time.Sleep(10 * time.Millisecond)
      continue
    }
    if synch.wasAnyOperationErroneous() {
      return synch.mergeErrorMessages()
    }
    return nil
  }
  return errors.New("CNI operation timed-out after " + strconv.Itoa(MaximumAllowedTime) + " seconds")
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
