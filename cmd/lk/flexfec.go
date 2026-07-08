// Copyright 2026 Ultra Robotics
//
// Adds publisher-side FlexFEC-03 to `lk room join --publish`.
//
// The FlexFEC encoder interceptor sits ahead of the SDK's default NACK / TWCC
// interceptors and emits a parallel FEC RTP stream on a separate SSRC for each
// outbound video track. The corresponding `video/flexfec-03` codec must also
// be present on the MediaEngine — otherwise Pion never populates
// StreamInfo.PayloadTypeForwardErrorCorrection and the interceptor stays idle.
//
// The codec list passed via `WithCodecs` keeps the codecs LiveKit publishes by
// default (H.264 baseline+main+high, H.265, VP8, VP9, AV1, Opus, RTX, ULPFEC,
// audio RED) and just adds flexfec-03. The protocol fork at
// github.com/ultra-robotics/livekit-protocol branch ultra/flexfec adds the
// flexfec-03 mime to `codecs.ToWebrtcCodecParameters` so that mime survives
// the livekit.Codec → webrtc.RTPCodecParameters conversion.

package main

import (
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/flexfec"
	"github.com/pion/webrtc/v4"
	"github.com/urfave/cli/v3"

	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
)

func buildFlexFECConnectOptions(cmd *cli.Command) ([]lksdk.ConnectOption, error) {
	fecFactory, err := flexfec.NewFecInterceptor(
		flexfec.NumMediaPackets(uint32(cmd.Uint("flexfec-media-packets"))),
		flexfec.NumFECPackets(uint32(cmd.Uint("flexfec-packets"))),
	)
	if err != nil {
		return nil, err
	}

	// The order below matters loosely (server-sdk-go re-orders into the
	// MediaEngine in registration order). Keep H.264 first so it's the
	// preferred outbound codec when the SDK creates an H.264 track.
	codecs := []livekit.Codec{
		{Mime: webrtc.MimeTypeH264, FmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f"},
		{Mime: webrtc.MimeTypeH265},
		{Mime: webrtc.MimeTypeVP8},
		{Mime: webrtc.MimeTypeVP9},
		{Mime: webrtc.MimeTypeAV1},
		{Mime: webrtc.MimeTypeRTX},
		{Mime: webrtc.MimeTypeFlexFEC03, FmtpLine: "repair-window=10000000"},
		{Mime: webrtc.MimeTypeOpus},
	}

	return []lksdk.ConnectOption{
		lksdk.WithCodecs(codecs),
		lksdk.WithInterceptors([]interceptor.Factory{fecFactory}),
		// The Pion FlexFEC blog is explicit: register FlexFEC before any
		// interceptor that modifies RTP packets (TWCC etc.), so default
		// interceptors must come *after* in the registry. WithIncludeDefault-
		// Interceptors=true accomplishes that — defaults are appended after
		// our custom interceptors.
		lksdk.WithIncludeDefaultInterceptors(true),
	}, nil
}
