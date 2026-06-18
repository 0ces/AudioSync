//go:build darwin

#include "tap_darwin.h"

#import <Foundation/Foundation.h>
#import <CoreAudio/CoreAudio.h>
#import <CoreAudio/AudioHardwareTapping.h>
#import <CoreAudio/CATapDescription.h>
#include <stdatomic.h>
#include <string.h>

// ---- lock-free SPSC byte ring (C side) -------------------------------------
// Sized to ~1.3s of 48kHz stereo S16 so a slow Go drain never corrupts audio.
#define RING_CAP (1u << 18) // 262144, power of two
static uint8_t g_ring[RING_CAP];
static _Atomic uint64_t g_head; // bytes written (producer: IOProc)
static _Atomic uint64_t g_tail; // bytes read (consumer: Go)

static void ring_write(const uint8_t *src, int n) {
    uint64_t head = atomic_load_explicit(&g_head, memory_order_relaxed);
    uint64_t tail = atomic_load_explicit(&g_tail, memory_order_acquire);
    int free = (int)(RING_CAP - (head - tail));
    if (n > free) n = free; // drop on overflow rather than block the RT thread
    uint32_t idx = (uint32_t)(head & (RING_CAP - 1));
    int first = (int)(RING_CAP - idx);
    if (first > n) first = n;
    memcpy(&g_ring[idx], src, first);
    if (n > first) memcpy(&g_ring[0], src + first, n - first);
    atomic_store_explicit(&g_head, head + (uint64_t)n, memory_order_release);
}

int audiosync_tap_read(uint8_t *dst, int cap) {
    uint64_t tail = atomic_load_explicit(&g_tail, memory_order_relaxed);
    uint64_t head = atomic_load_explicit(&g_head, memory_order_acquire);
    int avail = (int)(head - tail);
    int n = cap < avail ? cap : avail;
    if (n <= 0) return 0;
    uint32_t idx = (uint32_t)(tail & (RING_CAP - 1));
    int first = (int)(RING_CAP - idx);
    if (first > n) first = n;
    memcpy(dst, &g_ring[idx], first);
    if (n > first) memcpy(dst + first, &g_ring[0], n - first);
    atomic_store_explicit(&g_tail, tail + (uint64_t)n, memory_order_release);
    return n;
}

// ---- CoreAudio tap state ---------------------------------------------------
static AudioObjectID g_tap = kAudioObjectUnknown;
static AudioDeviceID g_agg = kAudioObjectUnknown;
static AudioDeviceIOProcID g_proc = NULL;
static UInt32 g_channels = 2;
static int g_nonInterleaved = 0;

static inline int16_t f32_to_s16(float v) {
    if (v > 1.0f) v = 1.0f;
    else if (v < -1.0f) v = -1.0f;
    return (int16_t)(v * 32767.0f);
}

// MAXF caps frames per callback so the RT-stack scratch is a fixed-size array.
enum { MAXF = 8192 };

// Converts one IOProc input buffer list (Float32) to interleaved S16 stereo.
static void convert_and_push(const AudioBufferList *in) {
    if (in->mNumberBuffers == 0) return;

    // Scratch on the RT stack (32 KiB), bounded by MAXF.
    int16_t out[MAXF * 2];

    if (g_nonInterleaved) {
        // Planar: buffer[c] holds Float32 frames for channel c.
        int frames = (int)(in->mBuffers[0].mDataByteSize / sizeof(float));
        if (frames > MAXF) frames = MAXF;
        const float *L = (const float *)in->mBuffers[0].mData;
        const float *R = (in->mNumberBuffers > 1)
                             ? (const float *)in->mBuffers[1].mData
                             : L;
        for (int i = 0; i < frames; i++) {
            out[2 * i] = f32_to_s16(L[i]);
            out[2 * i + 1] = f32_to_s16(R[i]);
        }
        ring_write((const uint8_t *)out, frames * 4);
    } else {
        // Interleaved: single buffer, ch-major within each frame.
        const float *src = (const float *)in->mBuffers[0].mData;
        int ch = (int)g_channels ? (int)g_channels : 1;
        int frames = (int)(in->mBuffers[0].mDataByteSize / (sizeof(float) * ch));
        if (frames > MAXF) frames = MAXF;
        for (int i = 0; i < frames; i++) {
            float l = src[i * ch];
            float r = (ch > 1) ? src[i * ch + 1] : l;
            out[2 * i] = f32_to_s16(l);
            out[2 * i + 1] = f32_to_s16(r);
        }
        ring_write((const uint8_t *)out, frames * 4);
    }
}

int audiosync_tap_start(uint32_t *outSampleRate) {
    @autoreleasepool {
        // Process taps require macOS 14.2+. Everything below runs inside this
        // availability check so the symbols are safe under the (lower)
        // deployment target the build sets.
        if (@available(macOS 14.2, *)) {
        // Global stereo tap of all processes (exclude none). Unmuted so the
        // sender machine still hears its own audio locally.
        CATapDescription *desc =
            [[CATapDescription alloc] initStereoGlobalTapButExcludeProcesses:@[]];
        desc.name = @"AudioSync System Tap";
        desc.privateTap = YES;
        desc.muteBehavior = CATapUnmuted;

        OSStatus st = AudioHardwareCreateProcessTap(desc, &g_tap);
        if (st != noErr || g_tap == kAudioObjectUnknown) return -1;

        // Query the tap's stream format (sample rate, channels, flags).
        AudioStreamBasicDescription asbd;
        UInt32 sz = sizeof(asbd);
        AudioObjectPropertyAddress fmtAddr = {
            kAudioTapPropertyFormat, kAudioObjectPropertyScopeGlobal,
            kAudioObjectPropertyElementMain};
        st = AudioObjectGetPropertyData(g_tap, &fmtAddr, 0, NULL, &sz, &asbd);
        if (st != noErr) {
            AudioHardwareDestroyProcessTap(g_tap);
            g_tap = kAudioObjectUnknown;
            return -2;
        }
        g_channels = asbd.mChannelsPerFrame ? asbd.mChannelsPerFrame : 2;
        g_nonInterleaved =
            (asbd.mFormatFlags & kAudioFormatFlagIsNonInterleaved) ? 1 : 0;
        if (outSampleRate) *outSampleRate = (uint32_t)asbd.mSampleRate;

        // Private aggregate device containing only this tap.
        NSString *tapUID = desc.UUID.UUIDString;
        NSDictionary *aggDict = @{
            @(kAudioAggregateDeviceNameKey) : @"AudioSyncAggregate",
            @(kAudioAggregateDeviceUIDKey) : @"com.audiosync.aggregate",
            @(kAudioAggregateDeviceIsPrivateKey) : @YES,
            @(kAudioAggregateDeviceIsStackedKey) : @NO,
            @(kAudioAggregateDeviceTapAutoStartKey) : @YES,
            @(kAudioAggregateDeviceTapListKey) : @[ @{
                @(kAudioSubTapUIDKey) : tapUID,
                @(kAudioSubTapDriftCompensationKey) : @YES,
            } ],
        };
        st = AudioHardwareCreateAggregateDevice(
            (__bridge CFDictionaryRef)aggDict, &g_agg);
        if (st != noErr || g_agg == kAudioObjectUnknown) {
            AudioHardwareDestroyProcessTap(g_tap);
            g_tap = kAudioObjectUnknown;
            return -3;
        }

        // Install the realtime IOProc.
        st = AudioDeviceCreateIOProcIDWithBlock(
            &g_proc, g_agg, NULL,
            ^(const AudioTimeStamp *inNow, const AudioBufferList *inInputData,
              const AudioTimeStamp *inInputTime, AudioBufferList *outOutputData,
              const AudioTimeStamp *inOutputTime) {
                (void)inNow;
                (void)inInputTime;
                (void)outOutputData;
                (void)inOutputTime;
                convert_and_push(inInputData);
            });
        if (st != noErr) {
            AudioHardwareDestroyAggregateDevice(g_agg);
            AudioHardwareDestroyProcessTap(g_tap);
            g_agg = kAudioObjectUnknown;
            g_tap = kAudioObjectUnknown;
            return -4;
        }

        st = AudioDeviceStart(g_agg, g_proc);
        if (st != noErr) {
            audiosync_tap_stop();
            return -5;
        }
        return 0;
        } else {
            return -10;
        }
    }
}

void audiosync_tap_stop(void) {
    if (g_agg != kAudioObjectUnknown && g_proc) {
        AudioDeviceStop(g_agg, g_proc);
        AudioDeviceDestroyIOProcID(g_agg, g_proc);
        g_proc = NULL;
    }
    if (g_agg != kAudioObjectUnknown) {
        AudioHardwareDestroyAggregateDevice(g_agg);
        g_agg = kAudioObjectUnknown;
    }
    if (g_tap != kAudioObjectUnknown) {
        if (@available(macOS 14.2, *)) {
            AudioHardwareDestroyProcessTap(g_tap);
        }
        g_tap = kAudioObjectUnknown;
    }
    // Reset ring so a restart starts clean.
    atomic_store_explicit(&g_head, 0, memory_order_relaxed);
    atomic_store_explicit(&g_tail, 0, memory_order_relaxed);
}
