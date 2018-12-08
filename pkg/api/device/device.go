package device

import "time"

// Device represents a managed device
type Device struct {
	// DeviceID  is the id for this device.
	DeviceID             string `json:"deviceId"`
	DeviceType           string `json:"deviceType"`
	Name                 string `json:"deviceName"`
	Location             string `json:"location"`
	AssetTag             string `json:"assetTag"`
	Group                string `json:"group"`
	HardwareModel        string `json:"hardwareModel"`
	HardwareRevision     string `json:"hardwareRevision"`
	HardwareSerialNumber string `json:"hardwareSerialNumber"`
	FirmwareName         string `json:"firmwareName"`
	FirmwareVersion      string `json:"firmwareVersion"`

	// CreatedAt returns the timestamp of the client's creation.
	CreatedAt time.Time `json:"created_at,omitempty"`

	// UpdatedAt returns the timestamp of the last update.
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

func (d *Device) GetID() string {
	return d.DeviceID
}
