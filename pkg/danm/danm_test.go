package main

import (
  "testing"
)

var devicePool0 = "pool0"
var devicePool1 = "pool1"

var allocatedDevices = make(map[string]*[]string)

func TestPopDevicePanic(t *testing.T) {
    defer func() {
        if r := recover(); r == nil {
            t.Errorf("Uninitialized map should expect error.")
        }
    }()

    popDevice(devicePool0, allocatedDevices)
}

func TestPopDevice(t *testing.T) {  
  allocatedDevices[devicePool0] = &[]string{"device0"}
  allocatedDevices[devicePool1] = &[]string{"device1", "device2"}
  
  device,err := popDevice(devicePool1, allocatedDevices)
  if device != "device2"  || err != nil {
    t.Errorf("Received device or error does not match with expectation.")
  }
  device,err = popDevice(devicePool1, allocatedDevices)
  if device != "device1"  || err != nil {
    t.Errorf("Received device or error does not match with expectation.")
  }
  device,err = popDevice(devicePool1, allocatedDevices)
  if device == ""  && err == nil {
    t.Errorf("Empty pool should expect error.")
  }
}
