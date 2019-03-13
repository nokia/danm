package syncher_test

import (
  "errors"
  "testing"
  "time"
  "github.com/containernetworking/cni/pkg/types/current"
  "github.com/nokia/danm/pkg/syncher"
)
const (
  timeout = 10
)

type result struct {
  cniName string
  opRes error
  cniRes *current.Result
}

var resultInterfaces = []*current.Interface {
      &current.Interface{Name: "eth0",},
      &current.Interface{Name: "eth1",},
      &current.Interface{Name: "fastpath",},
    }

var failingTestConsts = []result {
  {"sriov", nil, &current.Result{CNIVersion: "0.3.1", Interfaces: resultInterfaces },},
  {"flannel", errors.New("this did not go well"), nil},
  {"ipvlan", errors.New("neither did this"), &current.Result{CNIVersion: "0.3.1", Interfaces: resultInterfaces },},
}

var totalSuccessTestConsts = []result {
  {"sriov", nil, &current.Result{CNIVersion: "0.3.1", Interfaces: resultInterfaces },},
  {"flannel", nil, nil},
  {"ipvlan", nil, &current.Result{CNIVersion: "0.4.0", Interfaces: resultInterfaces },},
}

func setupTest(expectedNumber int, results []result) *syncher.Syncher {
  numberOfResults := expectedNumber
  syncher := syncher.NewSyncher(numberOfResults)
  for _, result := range results {
    syncher.PushResult(result.cniName, result.opRes, result.cniRes)
  }
  return syncher
}

func TestPushResult(t *testing.T) {
  syncher := setupTest(len(failingTestConsts), failingTestConsts)
  if syncher.ExpectedNumOfResults != len(failingTestConsts) {
    t.Errorf("Expected number of stored results in object:%d does not match with the initalized value:%d", syncher.ExpectedNumOfResults, len(failingTestConsts))
  }
  if len(syncher.CniResults) != len(failingTestConsts) {
    t.Errorf("Number of stored results in object:%d does not match with the number we have pushed:%d", len(syncher.CniResults), len(failingTestConsts))
  }
  for index, result := range failingTestConsts {
    t.Run(result.cniName, func(t *testing.T) {
      if syncher.CniResults[index].CniName != result.cniName {
        t.Errorf("CNI name attribute stored inside object:%s does not match with expected:%s", syncher.CniResults[index].CniName, result.cniName)
      }
      if syncher.CniResults[index].OpResult != result.opRes {
        t.Errorf("Operation result attribute stored inside object:%v does not match with expected:%v", syncher.CniResults[index].OpResult, result.opRes)
      }
      if syncher.CniResults[index].CniResult != result.cniRes {
        t.Errorf("CNI operation result attribute stored inside object does not match with expected")
      }
    })
  }
}

func TestGetAggregatedResultSuccess(t *testing.T) {
  syncher := setupTest(len(totalSuccessTestConsts)+1, totalSuccessTestConsts)
  go addResultToSyncher(syncher,result{"ipvlan", nil, nil})
  err := syncher.GetAggregatedResult()
  if err != nil {
    t.Errorf("Results could not be successfully aggregated against our expectation, because: %v", err) 
  }
}

func TestGetAggregatedResultFail(t *testing.T) {
  syncher := setupTest(len(totalSuccessTestConsts)+1, totalSuccessTestConsts)
  startTime := time.Now()
  go addResultToSyncher(syncher,result{"ipvlan", errors.New("not this time"), nil})
  err := syncher.GetAggregatedResult()
  endTime := time.Now()
  if err == nil {
    t.Errorf("Somehow results were successfully aggregated against our expectation. Magic.") 
  }
  timeDifference := endTime.Sub(startTime)
  diffInSeconds := int(timeDifference.Seconds())
  if diffInSeconds >= timeout - 1 {
    t.Errorf("We have failed with an unexpected timeout!") 
  }
}

func TestGetAggregatedResultTimeout(t *testing.T) {
  syncher := setupTest(len(totalSuccessTestConsts)+1, totalSuccessTestConsts)
  startTime := time.Now()
  err := syncher.GetAggregatedResult()
  endTime := time.Now()
  if err == nil {
    t.Errorf("Somehow results were successfully aggregated against our expectation. Magic.") 
  }
  timeDifference := endTime.Sub(startTime)
  diffInSeconds := int(timeDifference.Seconds())
  if diffInSeconds <= timeout - 1 {
    t.Errorf("We have failed earlier than the timeout value!") 
  }
}

func TestMergeCniResults(t *testing.T) {
  syncher := setupTest(len(failingTestConsts), failingTestConsts)
  cniResult := syncher.MergeCniResults()
  var expectedNumberOfCniInterfaces int
  for _, result := range failingTestConsts {
    if result.cniRes != nil {
      expectedNumberOfCniInterfaces = expectedNumberOfCniInterfaces + len(result.cniRes.Interfaces)
    }
  }
  if len(cniResult.Interfaces) != expectedNumberOfCniInterfaces {
    t.Errorf("Number of interfaces inside the aggregated CNI result:%d does not match with the expected:%d", len(cniResult.Interfaces), expectedNumberOfCniInterfaces)
  }
}

func TestWasAnyOperationErroneous(t *testing.T) {
  emptySyncher := setupTest(0, nil)
  if emptySyncher.WasAnyOperationErroneous() {
     t.Errorf("An empty Syncher object shall not think that any CNI operations were erroneous!")  
  }
  failSyncher := setupTest(len(failingTestConsts), failingTestConsts)
  if !failSyncher.WasAnyOperationErroneous() {
     t.Errorf("A Syncher object with failing operations shall not think that all CNI operations were successful!")  
  }
  successSyncher := setupTest(len(totalSuccessTestConsts), totalSuccessTestConsts)
  if successSyncher.WasAnyOperationErroneous() {
     t.Errorf("A Syncher object with only successful operations shall not think that any CNI operations failed!")  
  }
}

func addResultToSyncher(syncher *syncher.Syncher, res result) {
  time.Sleep(2 * time.Second)
  syncher.PushResult(res.cniName, res.opRes, res.cniRes)
}

