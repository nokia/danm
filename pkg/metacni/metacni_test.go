package metacni

import (
  "testing"
)

var devicePool0 = "pool0"
var devicePool1 = "pool1"

func TestPopDevice(t *testing.T) {
  allocatedDevices := make(map[string]*[]string)

  device,err := popDevice(devicePool0, allocatedDevices)
  if err == nil {
    t.Errorf("Uninitialized map should expect error.")
  }

  allocatedDevices[devicePool0] = &[]string{"device0"}
  allocatedDevices[devicePool1] = &[]string{"device1", "device2"}

  device,err = popDevice(devicePool1, allocatedDevices)
  if device != "device2"  || err != nil {
    t.Errorf("Received device or error does not match with expectation.")
  }
  device,err = popDevice(devicePool1, allocatedDevices)
  if device != "device1"  || err != nil {
    t.Errorf("Received device or error does not match with expectation.")
  }
  device,err = popDevice(devicePool1, allocatedDevices)
  if err == nil {
    t.Errorf("Empty pool should expect error.")
  }
}
