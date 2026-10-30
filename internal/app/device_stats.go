package app

import (
	"time"

	"my-weread-desk/internal/library"
	"my-weread-desk/internal/weread/device"
)

// DeviceReadStats is the local "this device" reading counter. WeRead's
// /readdata/detail endpoint only returns account-wide aggregates, so the
// per-client breakdown is maintained locally by the reporter.
type DeviceReadStats struct {
	Client       string                  `json:"client"`
	DeviceName   string                  `json:"deviceName"`
	DeviceID     string                  `json:"deviceId"`
	TotalSeconds int64                   `json:"totalSeconds"`
	TodaySeconds int64                   `json:"todaySeconds"`
	Daily        []library.DeviceReadDay `json:"daily"`
}

// DeviceStats returns the local per-device reading time and the identity this
// installation presents to WeRead.
func (a *App) DeviceStats() (*DeviceReadStats, error) {
	daily, err := a.store.DeviceReadTimes(90)
	if err != nil {
		return nil, err
	}
	total, err := a.store.DeviceReadTotal()
	if err != nil {
		return nil, err
	}

	today := time.Now().Format("2006-01-02")
	var todaySeconds int64
	for _, d := range daily {
		if d.Date == today {
			todaySeconds = d.Seconds
			break
		}
	}

	return &DeviceReadStats{
		Client:       device.ClientName,
		DeviceName:   device.Name,
		DeviceID:     maskDeviceID(a.session.Store().DeviceID()),
		TotalSeconds: total,
		TodaySeconds: todaySeconds,
		Daily:        daily,
	}, nil
}

func maskDeviceID(id string) string {
	if len(id) <= 6 {
		return id
	}
	return "..." + id[len(id)-6:]
}
