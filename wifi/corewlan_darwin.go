//go:build darwin

package wifi

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreWLAN -framework Foundation -framework CoreLocation

#import <CoreWLAN/CoreWLAN.h>
#import <CoreLocation/CoreLocation.h>
#include <stdlib.h>

// LocationDelegate handles CLLocationManager authorization callbacks.
@interface LocationDelegate : NSObject <CLLocationManagerDelegate>
@property (nonatomic, assign) BOOL authorized;
@property (nonatomic, assign) BOOL responded;
@end

@implementation LocationDelegate
- (void)locationManagerDidChangeAuthorization:(CLLocationManager *)manager {
	CLAuthorizationStatus status = [manager authorizationStatus];
	self.authorized = (status == kCLAuthorizationStatusAuthorizedAlways ||
	                   status == kCLAuthorizationStatusAuthorized);
	self.responded = YES;
}
@end

// requestLocationAuthorization requests location access and blocks until
// the user responds or the permission is already determined.
// Returns 1 if authorized, 0 otherwise.
static int requestLocationAuthorization(void) {
	@autoreleasepool {
		CLLocationManager *mgr = [[CLLocationManager alloc] init];
		CLAuthorizationStatus status = [mgr authorizationStatus];

		// Already determined.
		if (status == kCLAuthorizationStatusAuthorizedAlways ||
		    status == kCLAuthorizationStatusAuthorized) {
			return 1;
		}
		if (status == kCLAuthorizationStatusDenied ||
		    status == kCLAuthorizationStatusRestricted) {
			return 0;
		}

		// Not determined — request and wait.
		LocationDelegate *delegate = [[LocationDelegate alloc] init];
		mgr.delegate = delegate;
		[mgr requestWhenInUseAuthorization];

		// Spin the run loop briefly to let the delegate fire.
		NSDate *timeout = [NSDate dateWithTimeIntervalSinceNow:30.0];
		while (!delegate.responded && [[NSDate date] compare:timeout] == NSOrderedAscending) {
			[[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode
			                         beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.1]];
		}
		return delegate.authorized ? 1 : 0;
	}
}

// locationAuthorized checks if location is already authorized (non-blocking).
static int locationAuthorized(void) {
	@autoreleasepool {
		CLLocationManager *mgr = [[CLLocationManager alloc] init];
		CLAuthorizationStatus status = [mgr authorizationStatus];
		return (status == kCLAuthorizationStatusAuthorizedAlways ||
		        status == kCLAuthorizationStatusAuthorized) ? 1 : 0;
	}
}

typedef struct {
	const char *name;
	const char *hardwareAddress;
	int powerOn;
} CInterfaceInfo;

typedef struct {
	int connected;
	const char *ssid;
	const char *bssid;
	long rssi;
	long noise;
	long channel;
	int band;     // 0=unknown, 1=2.4GHz, 2=5GHz, 3=6GHz
	int security; // maps to SecurityType
	double txRate;
	int phyMode;  // maps to PHYMode
} CConnectionInfo;

typedef struct {
	const char *ssid;
	const char *bssid;
	long rssi;
	long channel;
	int band;
	int security;
} CNetworkInfo;

static char *copyNSString(NSString *s) {
	if (s == nil) return NULL;
	return strdup([s UTF8String]);
}

static int mapSecurity(CWSecurity sec) {
	switch (sec) {
	case kCWSecurityNone:           return 0;
	case kCWSecurityWEP:            return 1;
	case kCWSecurityWPAPersonal:    return 2;
	case kCWSecurityWPA2Personal:   return 3;
	case kCWSecurityWPA3Personal:   return 4;
	case kCWSecurityWPAEnterprise:  return 5;
	case kCWSecurityWPA2Enterprise: return 6;
	case kCWSecurityWPA3Enterprise: return 7;
	default:                        return 8; // Unknown
	}
}

// Probe CWNetwork for the best security it supports (highest first).
static int probeNetworkSecurity(CWNetwork *net) {
	if ([net supportsSecurity:kCWSecurityWPA3Enterprise]) return 7;
	if ([net supportsSecurity:kCWSecurityWPA2Enterprise]) return 6;
	if ([net supportsSecurity:kCWSecurityWPAEnterprise])  return 5;
	if ([net supportsSecurity:kCWSecurityWPA3Personal])   return 4;
	if ([net supportsSecurity:kCWSecurityWPA2Personal])   return 3;
	if ([net supportsSecurity:kCWSecurityWPAPersonal])    return 2;
	if ([net supportsSecurity:kCWSecurityWEP])            return 1;
	if ([net supportsSecurity:kCWSecurityNone])           return 0;
	return 8; // Unknown
}

static int mapBand(CWChannelBand band) {
	switch (band) {
	case kCWChannelBand2GHz: return 1;
	case kCWChannelBand5GHz: return 2;
	case kCWChannelBand6GHz: return 3;
	default:                 return 0;
	}
}

static int mapPHYMode(CWPHYMode mode) {
	switch (mode) {
	case kCWPHYMode11a:  return 1;
	case kCWPHYMode11b:  return 2;
	case kCWPHYMode11g:  return 3;
	case kCWPHYMode11n:  return 4;
	case kCWPHYMode11ac: return 5;
	case kCWPHYMode11ax: return 6;
	default:             return 0;
	}
}

CInterfaceInfo getInterfaceInfo(void) {
	CInterfaceInfo info = {0};
	@autoreleasepool {
		CWInterface *iface = [[CWWiFiClient sharedWiFiClient] interface];
		if (iface == nil) return info;
		info.name = copyNSString([iface interfaceName]);
		info.hardwareAddress = copyNSString([iface hardwareAddress]);
		info.powerOn = [iface powerOn] ? 1 : 0;
	}
	return info;
}

CConnectionInfo getConnectionInfo(void) {
	CConnectionInfo info = {0};
	@autoreleasepool {
		CWInterface *iface = [[CWWiFiClient sharedWiFiClient] interface];
		if (iface == nil || [iface ssid] == nil) return info;
		info.connected = 1;
		info.ssid = copyNSString([iface ssid]);
		info.bssid = copyNSString([iface bssid]);
		info.rssi = [iface rssiValue];
		info.noise = [iface noiseMeasurement];
		CWChannel *ch = [iface wlanChannel];
		if (ch != nil) {
			info.channel = [ch channelNumber];
			info.band = mapBand([ch channelBand]);
		}
		info.security = mapSecurity([iface security]);
		info.txRate = [iface transmitRate];
		info.phyMode = mapPHYMode([iface activePHYMode]);
	}
	return info;
}

int scanNetworks(CNetworkInfo *out, int maxCount, char **errOut) {
	@autoreleasepool {
		CWInterface *iface = [[CWWiFiClient sharedWiFiClient] interface];
		if (iface == nil) {
			if (errOut) *errOut = strdup("no WiFi interface found");
			return -1;
		}
		NSError *error = nil;
		NSSet<CWNetwork *> *networks = [iface scanForNetworksWithName:nil error:&error];
		if (error != nil) {
			if (errOut) *errOut = copyNSString([error localizedDescription]);
			return -1;
		}
		if (networks == nil) return 0;

		int count = 0;
		for (CWNetwork *net in networks) {
			if (count >= maxCount) break;
			out[count].ssid = copyNSString([net ssid]);
			out[count].bssid = copyNSString([net bssid]);
			out[count].rssi = [net rssiValue];
			CWChannel *ch = [net wlanChannel];
			if (ch != nil) {
				out[count].channel = [ch channelNumber];
				out[count].band = mapBand([ch channelBand]);
			}
			out[count].security = probeNetworkSecurity(net);
			count++;
		}
		return count;
	}
}
*/
import "C"

import (
	"errors"
	"sync"
	"unsafe"
)

const maxScanResults = 256

// WLANClient wraps CoreWLAN access. All methods are thread-safe.
type WLANClient struct {
	mu             sync.Mutex
	locationAuthed bool
}

// NewWLANClient returns a new CoreWLAN client.
// It requests Location Services authorization which is required for SSID visibility.
func NewWLANClient() *WLANClient {
	authed := C.requestLocationAuthorization() != 0
	return &WLANClient{locationAuthed: authed}
}

// LocationAuthorized reports whether Location Services access was granted.
func (w *WLANClient) LocationAuthorized() bool {
	return w.locationAuthed
}

func goString(cs *C.char) string {
	if cs == nil {
		return ""
	}
	s := C.GoString(cs)
	C.free(unsafe.Pointer(cs))
	return s
}

func bandString(band C.int) string {
	switch band {
	case 1:
		return "2.4 GHz"
	case 2:
		return "5 GHz"
	case 3:
		return "6 GHz"
	default:
		return ""
	}
}

// InterfaceInfo returns the WiFi interface state.
func (w *WLANClient) InterfaceInfo() (InterfaceInfo, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	ci := C.getInterfaceInfo()
	info := InterfaceInfo{
		Name:         goString(ci.name),
		HardwareAddr: goString(ci.hardwareAddress),
		PowerOn:      ci.powerOn != 0,
	}
	return info, nil
}

// ConnectionInfo returns details about the current WiFi connection.
func (w *WLANClient) ConnectionInfo() (ConnectionInfo, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	cc := C.getConnectionInfo()
	conn := ConnectionInfo{
		Connected: cc.connected != 0,
		SSID:      goString(cc.ssid),
		BSSID:     goString(cc.bssid),
		RSSI:      int(cc.rssi),
		Noise:     int(cc.noise),
		Channel:   int(cc.channel),
		Band:      bandString(cc.band),
		Security:  SecurityType(cc.security),
		TXRate:    float64(cc.txRate),
		PHYMode:   PHYMode(cc.phyMode),
	}
	return conn, nil
}

// ScanNetworks scans for available WiFi networks.
func (w *WLANClient) ScanNetworks() ([]NetworkInfo, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var buf [maxScanResults]C.CNetworkInfo
	var errStr *C.char

	count := C.scanNetworks(&buf[0], C.int(maxScanResults), &errStr)
	if count < 0 {
		msg := goString(errStr)
		return nil, errors.New(msg)
	}

	networks := make([]NetworkInfo, 0, int(count))
	for i := 0; i < int(count); i++ {
		cn := buf[i]
		networks = append(networks, NetworkInfo{
			SSID:     goString(cn.ssid),
			BSSID:    goString(cn.bssid),
			RSSI:     int(cn.rssi),
			Channel:  int(cn.channel),
			Band:     bandString(cn.band),
			Security: SecurityType(cn.security),
		})
	}
	return networks, nil
}
