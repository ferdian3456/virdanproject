// Package device holds the handler logic shared by every framework under test,
// so each framework only differs in routing and request/response plumbing.
package device

import (
	"encoding/json"
	"errors"
	"sync/atomic"
)

type Device struct {
	ID       int64  `json:"id"`
	MAC      string `json:"mac"`
	Firmware string `json:"firmware"`
}

var ErrInvalid = errors.New("mac and firmware are required")

var sample = []Device{
	{ID: 1, MAC: "5F-33-CC-1F-43-82", Firmware: "2.1.6"},
	{ID: 2, MAC: "44-39-34-5E-9C-F2", Firmware: "3.0.1"},
	{ID: 3, MAC: "2B-6E-79-C7-22-1B", Firmware: "1.8.9"},
	{ID: 4, MAC: "06-0A-79-47-18-E1", Firmware: "4.0.9"},
	{ID: 5, MAC: "68-32-8F-00-B6-F4", Firmware: "5.0.0"},
}

var lastID atomic.Int64

// List returns the JSON encoding of the sample devices. It encodes on every
// call on purpose: serialization cost is part of what we measure.
func List() ([]byte, error) {
	return json.Marshal(sample)
}

// Create decodes a device from body, assigns it an ID and returns its JSON encoding.
func Create(body []byte) ([]byte, error) {
	var d Device
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, err
	}
	if d.MAC == "" || d.Firmware == "" {
		return nil, ErrInvalid
	}
	d.ID = lastID.Add(1)
	return json.Marshal(d)
}
