//go:build darwin

package audio

/*
#cgo LDFLAGS: -framework CoreAudio -framework CoreFoundation
#include <CoreAudio/CoreAudio.h>
#include <CoreFoundation/CoreFoundation.h>
#include <libproc.h>
#include <stdlib.h>

// Forward declarations.
static int cfStringToCString(CFStringRef cfStr, char *buf, int bufLen);

// audioProcess holds PID, object ID, and state for an audio-producing process.
typedef struct {
	AudioObjectID objectID;
	pid_t         pid;
	int           isRunningOutput;
	char          bundleID[256];
} AudioProcessInfo;

// listAudioProcesses returns the number of audio-producing processes and fills buf.
// Returns 0 on error or if the property is unavailable.
static int listAudioProcesses(AudioProcessInfo *buf, int bufLen) {
	AudioObjectPropertyAddress addr = {
		.mSelector = kAudioHardwarePropertyProcessObjectList,
		.mScope    = kAudioObjectPropertyScopeGlobal,
		.mElement  = kAudioObjectPropertyElementMain,
	};

	UInt32 dataSize = 0;
	OSStatus status = AudioObjectGetPropertyDataSize(
		kAudioObjectSystemObject, &addr, 0, NULL, &dataSize);
	if (status != noErr || dataSize == 0) {
		return 0;
	}

	int count = (int)(dataSize / sizeof(AudioObjectID));
	if (count > bufLen) {
		count = bufLen;
	}

	AudioObjectID *objectIDs = (AudioObjectID *)malloc(dataSize);
	if (!objectIDs) {
		return 0;
	}

	status = AudioObjectGetPropertyData(
		kAudioObjectSystemObject, &addr, 0, NULL, &dataSize, objectIDs);
	if (status != noErr) {
		free(objectIDs);
		return 0;
	}

	count = (int)(dataSize / sizeof(AudioObjectID));
	if (count > bufLen) {
		count = bufLen;
	}

	for (int i = 0; i < count; i++) {
		buf[i].objectID = objectIDs[i];
		buf[i].isRunningOutput = 0;
		buf[i].bundleID[0] = '\0';

		// Get PID.
		AudioObjectPropertyAddress pidAddr = {
			.mSelector = kAudioProcessPropertyPID,
			.mScope    = kAudioObjectPropertyScopeGlobal,
			.mElement  = kAudioObjectPropertyElementMain,
		};
		pid_t pid = 0;
		UInt32 pidSize = sizeof(pid);
		OSStatus pidStatus = AudioObjectGetPropertyData(
			objectIDs[i], &pidAddr, 0, NULL, &pidSize, &pid);
		buf[i].pid = (pidStatus == noErr) ? pid : 0;

		// Check if process is actively producing audio output.
		AudioObjectPropertyAddress runAddr = {
			.mSelector = kAudioProcessPropertyIsRunningOutput,
			.mScope    = kAudioObjectPropertyScopeGlobal,
			.mElement  = kAudioObjectPropertyElementMain,
		};
		UInt32 isRunning = 0;
		UInt32 runSize = sizeof(isRunning);
		OSStatus runStatus = AudioObjectGetPropertyData(
			objectIDs[i], &runAddr, 0, NULL, &runSize, &isRunning);
		buf[i].isRunningOutput = (runStatus == noErr && isRunning) ? 1 : 0;

		// Get bundle ID.
		AudioObjectPropertyAddress bidAddr = {
			.mSelector = kAudioProcessPropertyBundleID,
			.mScope    = kAudioObjectPropertyScopeGlobal,
			.mElement  = kAudioObjectPropertyElementMain,
		};
		CFStringRef cfBundleID = NULL;
		UInt32 bidSize = sizeof(cfBundleID);
		OSStatus bidStatus = AudioObjectGetPropertyData(
			objectIDs[i], &bidAddr, 0, NULL, &bidSize, &cfBundleID);
		if (bidStatus == noErr && cfBundleID) {
			cfStringToCString(cfBundleID, buf[i].bundleID, 256);
			CFRelease(cfBundleID);
		}
	}

	free(objectIDs);
	return count;
}

// getProcessName fills name with the process name for a given PID.
// Returns the length of the name, or 0 on error.
static int getProcessName(pid_t pid, char *name, int nameLen) {
	int ret = proc_name(pid, name, nameLen);
	return ret;
}

// AudioDeviceInfo holds device enumeration results.
typedef struct {
	AudioObjectID deviceID;
	int           isDefault;
	char          name[256];
	char          uid[256];
} AudioDeviceInfo;

// cfStringToCString copies a CFStringRef into a C buffer. Returns 0 on success.
static int cfStringToCString(CFStringRef cfStr, char *buf, int bufLen) {
	if (!cfStr) return -1;
	if (CFStringGetCString(cfStr, buf, bufLen, kCFStringEncodingUTF8)) {
		return 0;
	}
	return -1;
}

// hasOutputStreams checks if a device has output streams.
static int hasOutputStreams(AudioObjectID deviceID) {
	AudioObjectPropertyAddress addr = {
		.mSelector = kAudioDevicePropertyStreams,
		.mScope    = kAudioObjectPropertyScopeOutput,
		.mElement  = kAudioObjectPropertyElementMain,
	};
	UInt32 dataSize = 0;
	OSStatus status = AudioObjectGetPropertyDataSize(deviceID, &addr, 0, NULL, &dataSize);
	if (status != noErr) return 0;
	return (dataSize > 0) ? 1 : 0;
}

// getDeviceName gets the name of an audio device.
static int getDeviceName(AudioObjectID deviceID, char *name, int nameLen) {
	AudioObjectPropertyAddress addr = {
		.mSelector = kAudioObjectPropertyName,
		.mScope    = kAudioObjectPropertyScopeGlobal,
		.mElement  = kAudioObjectPropertyElementMain,
	};
	CFStringRef cfName = NULL;
	UInt32 dataSize = sizeof(cfName);
	OSStatus status = AudioObjectGetPropertyData(deviceID, &addr, 0, NULL, &dataSize, &cfName);
	if (status != noErr || !cfName) return -1;
	int ret = cfStringToCString(cfName, name, nameLen);
	CFRelease(cfName);
	return ret;
}

// getDeviceUID gets the UID of an audio device.
static int getDeviceUID(AudioObjectID deviceID, char *uid, int uidLen) {
	AudioObjectPropertyAddress addr = {
		.mSelector = kAudioDevicePropertyDeviceUID,
		.mScope    = kAudioObjectPropertyScopeGlobal,
		.mElement  = kAudioObjectPropertyElementMain,
	};
	CFStringRef cfUID = NULL;
	UInt32 dataSize = sizeof(cfUID);
	OSStatus status = AudioObjectGetPropertyData(deviceID, &addr, 0, NULL, &dataSize, &cfUID);
	if (status != noErr || !cfUID) return -1;
	int ret = cfStringToCString(cfUID, uid, uidLen);
	CFRelease(cfUID);
	return ret;
}

// getDefaultOutputDevice returns the default output device ID.
static AudioObjectID getDefaultOutputDevice() {
	AudioObjectPropertyAddress addr = {
		.mSelector = kAudioHardwarePropertyDefaultOutputDevice,
		.mScope    = kAudioObjectPropertyScopeGlobal,
		.mElement  = kAudioObjectPropertyElementMain,
	};
	AudioObjectID deviceID = 0;
	UInt32 dataSize = sizeof(deviceID);
	OSStatus status = AudioObjectGetPropertyData(
		kAudioObjectSystemObject, &addr, 0, NULL, &dataSize, &deviceID);
	if (status != noErr) return 0;
	return deviceID;
}

// listOutputDevices enumerates output devices. Returns the count.
static int listOutputDevices(AudioDeviceInfo *buf, int bufLen) {
	AudioObjectPropertyAddress addr = {
		.mSelector = kAudioHardwarePropertyDevices,
		.mScope    = kAudioObjectPropertyScopeGlobal,
		.mElement  = kAudioObjectPropertyElementMain,
	};

	UInt32 dataSize = 0;
	OSStatus status = AudioObjectGetPropertyDataSize(
		kAudioObjectSystemObject, &addr, 0, NULL, &dataSize);
	if (status != noErr || dataSize == 0) return 0;

	int totalDevices = (int)(dataSize / sizeof(AudioObjectID));
	AudioObjectID *deviceIDs = (AudioObjectID *)malloc(dataSize);
	if (!deviceIDs) return 0;

	status = AudioObjectGetPropertyData(
		kAudioObjectSystemObject, &addr, 0, NULL, &dataSize, deviceIDs);
	if (status != noErr) {
		free(deviceIDs);
		return 0;
	}

	totalDevices = (int)(dataSize / sizeof(AudioObjectID));
	AudioObjectID defaultID = getDefaultOutputDevice();

	int count = 0;
	for (int i = 0; i < totalDevices && count < bufLen; i++) {
		if (!hasOutputStreams(deviceIDs[i])) continue;

		buf[count].deviceID = deviceIDs[i];
		buf[count].isDefault = (deviceIDs[i] == defaultID) ? 1 : 0;

		if (getDeviceName(deviceIDs[i], buf[count].name, 256) != 0) {
			buf[count].name[0] = '\0';
		}
		if (getDeviceUID(deviceIDs[i], buf[count].uid, 256) != 0) {
			buf[count].uid[0] = '\0';
		}

		count++;
	}

	free(deviceIDs);
	return count;
}

// setDefaultOutputDevice sets the default output device.
static int setDefaultOutputDevice(AudioObjectID deviceID) {
	AudioObjectPropertyAddress addr = {
		.mSelector = kAudioHardwarePropertyDefaultOutputDevice,
		.mScope    = kAudioObjectPropertyScopeGlobal,
		.mElement  = kAudioObjectPropertyElementMain,
	};
	OSStatus status = AudioObjectSetPropertyData(
		kAudioObjectSystemObject, &addr, 0, NULL, sizeof(deviceID), &deviceID);
	return (status == noErr) ? 0 : (int)status;
}
*/
import "C"

import (
	"fmt"
	"strings"
)

// audioProcess represents a macOS process producing audio.
type audioProcess struct {
	ObjectID        uint32
	PID             int
	Name            string
	BundleID        string
	IsRunningOutput bool
}

// coreAudioDevice represents a macOS audio output device.
type coreAudioDevice struct {
	ID        uint32
	UID       string
	Name      string
	IsDefault bool
}

// coreAudioListProcesses returns all processes currently producing audio.
// Requires macOS 14+ (Sonoma). Returns empty slice on older systems.
func coreAudioListProcesses() ([]audioProcess, error) {
	const maxProcesses = 128
	var buf [maxProcesses]C.AudioProcessInfo

	count := C.listAudioProcesses(&buf[0], C.int(maxProcesses))
	if count <= 0 {
		return nil, nil
	}

	processes := make([]audioProcess, 0, int(count))
	for i := 0; i < int(count); i++ {
		pid := int(buf[i].pid)
		if pid <= 0 {
			continue
		}

		bundleID := C.GoString(&buf[i].bundleID[0])
		isRunning := buf[i].isRunningOutput != 0

		// Derive display name: prefer bundle ID mapping, fall back to proc_name.
		name := bundleIDToDisplayName(bundleID)
		if name == "" {
			var nameBuf [256]C.char
			nameLen := C.getProcessName(buf[i].pid, &nameBuf[0], 256)
			if nameLen > 0 {
				name = cleanProcessName(C.GoString(&nameBuf[0]))
			} else {
				name = "Unknown"
			}
		}

		processes = append(processes, audioProcess{
			ObjectID:        uint32(buf[i].objectID),
			PID:             pid,
			Name:            name,
			BundleID:        bundleID,
			IsRunningOutput: isRunning,
		})
	}
	return processes, nil
}

// coreAudioListOutputDevices returns all audio output devices.
func coreAudioListOutputDevices() ([]coreAudioDevice, error) {
	const maxDevices = 64
	var buf [maxDevices]C.AudioDeviceInfo

	count := C.listOutputDevices(&buf[0], C.int(maxDevices))
	if count <= 0 {
		return nil, fmt.Errorf("no output devices found")
	}

	devices := make([]coreAudioDevice, 0, int(count))
	for i := 0; i < int(count); i++ {
		name := C.GoString(&buf[i].name[0])
		uid := C.GoString(&buf[i].uid[0])
		if name == "" {
			continue
		}

		devices = append(devices, coreAudioDevice{
			ID:        uint32(buf[i].deviceID),
			UID:       uid,
			Name:      name,
			IsDefault: buf[i].isDefault != 0,
		})
	}
	return devices, nil
}

// coreAudioSetDefaultOutput sets the system default output device by device ID.
func coreAudioSetDefaultOutput(deviceID uint32) error {
	ret := C.setDefaultOutputDevice(C.AudioObjectID(deviceID))
	if ret != 0 {
		return fmt.Errorf("failed to set default output device (status %d)", int(ret))
	}
	return nil
}

// bundleIDToDisplayName maps known bundle IDs to user-friendly display names.
func bundleIDToDisplayName(bundleID string) string {
	known := map[string]string{
		"com.apple.Music":                "Music",
		"com.apple.Safari":               "Safari",
		"com.apple.TV":                   "TV",
		"com.apple.Podcasts":             "Podcasts",
		"com.apple.QuickTimePlayerX":     "QuickTime",
		"com.apple.iWork.Keynote":        "Keynote",
		"com.apple.FaceTime":             "FaceTime",
		"com.spotify.client":             "Spotify",
		"com.google.Chrome":              "Chrome",
		"com.google.Chrome.helper":       "Chrome",
		"org.mozilla.firefox":            "Firefox",
		"com.microsoft.edgemac":          "Edge",
		"com.brave.Browser":              "Brave",
		"us.zoom.xos":                    "Zoom",
		"com.microsoft.teams2":           "Teams",
		"com.hnc.Discord":               "Discord",
		"com.electron.replit":            "Replit",
		"com.tinyspeck.slackmacgap":     "Slack",
		"tv.twitch.studio":              "Twitch",
		"com.valvesoftware.steam":       "Steam",
		"com.apple.systempreferences":   "System Settings",
		"com.apple.VoiceMemos":          "Voice Memos",
		"com.apple.garageband":          "GarageBand",
		"com.apple.Logic10":             "Logic Pro",
	}
	if name, ok := known[bundleID]; ok {
		return name
	}
	// Try extracting the last component of the bundle ID as a fallback.
	if bundleID != "" {
		parts := strings.Split(bundleID, ".")
		if len(parts) > 0 {
			last := parts[len(parts)-1]
			if last != "helper" && last != "Helper" && len(last) > 1 {
				return last
			}
		}
	}
	return ""
}

// cleanProcessName normalizes a macOS process name for display.
func cleanProcessName(name string) string {
	// Remove "Helper" suffixes common in browser sub-processes.
	name = strings.TrimSuffix(name, " Helper")
	name = strings.TrimSuffix(name, " Helper (Renderer)")
	name = strings.TrimSuffix(name, " Helper (GPU)")
	return name
}
