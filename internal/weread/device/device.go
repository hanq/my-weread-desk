// Package device holds the E-Ink client identity. Both the login handshake and
// the API session present the same device to the server; a mismatch between
// them makes the minted access token look stolen and is rejected with
// "鉴权失败" (-2013).
package device

const (
	// Name is the device name sent on login/refresh. It stays the official
	// E-Ink model so the token is not treated as a different device.
	Name = "BOOX"
	// ClientName is the product identifier shown in the desktop UI and in the
	// local per-device reading statistics. It is deliberately not sent to
	// WeRead: UA, deviceType and deviceId are protocol identity, while this
	// name only labels the local client.
	ClientName = "MyWeReadDesk"
	// Type is the Android device class the official client reports.
	Type = 3
	// AppVersion is the official client version.
	AppVersion = "2.1.2.10245900"
	// UserAgent is the official E-Ink client UA.
	UserAgent = "WeRead/2.1.2 WRBrand/Onyx wr_eink Dalvik/2.1.0 (Linux; U; Android 11; BOOX Build/onyx)"
	// ChannelID is the distribution channel the official client uses.
	ChannelID = "900"
	// BaseAPI is the protocol revision.
	BaseAPI = "30"
	// OSVersion is the Android version reported alongside the UA.
	OSVersion = "11"
	// DeviceIDPrefix is the 13-character prefix every E-Ink device id starts
	// with; the remaining 19 characters are digits.
	DeviceIDPrefix = "eink334691225"
)

// Headers returns the version headers every E-Ink request carries.
func Headers() map[string]string {
	return map[string]string{
		"baseapi":   BaseAPI,
		"appver":    AppVersion,
		"basever":   AppVersion,
		"osver":     OSVersion,
		"channelId": ChannelID,
		"Accept":    "application/json, text/plain, */*",
	}
}
