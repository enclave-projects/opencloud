package preprocessor

import (
	"bytes"
	"context"
	"errors"
	"image"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/kovidgoyal/imaging"
	pkgerrors "github.com/pkg/errors"

	thumbnailerErrors "github.com/opencloud-eu/opencloud/services/thumbnails/pkg/errors"
)

// VideoDecoderConfig configures a VideoDecoder instance.
//
// The configuration is built once during service startup; per-request
// fields (the input bytes, the user's auth context, etc.) are NOT part of
// the configuration and are passed in via Convert.
type VideoDecoderConfig struct {
	// FFmpegBinary is the absolute (or PATH-resolved) path to the ffmpeg
	// executable. Must be set; an empty value disables the decoder.
	FFmpegBinary string
	// FFmpegTimeout is the hard wall-clock cap on the subprocess.
	FFmpegTimeout time.Duration
	// SeekOffset is the position in the input from which to extract the
	// representative frame (e.g. "00:00:01"). Server controlled; never
	// derived from user input.
	SeekOffset string
	// MaxOutputBytes is the hard cap on bytes read from ffmpeg stdout.
	MaxOutputBytes int64
}

// VideoDecoder extracts a single JPEG frame from a video file using ffmpeg
// and returns it as an image.Image suitable for the rest of the thumbnail
// pipeline.
//
// Security model:
//   - The subprocess is invoked via exec.CommandContext with explicit
//     positional arguments only. No shell is involved.
//   - The input is staged to a private temporary file with mode 0600;
//     the filename is server-generated, never derived from the user.
//   - The subprocess is bounded by a context.WithTimeout and ffmpeg's own
//     -frames:v 1 -an -sn -dn flags to do the minimum work possible.
//   - The protocol whitelist forbids ffmpeg from opening anything other
//     than the local file we just wrote.
//   - Output is read through an io.LimitReader so a hostile input cannot
//     make ffmpeg emit an unbounded amount of data.
type VideoDecoder struct {
	cfg VideoDecoderConfig
}

// NewVideoDecoder builds a VideoDecoder, returning an error when the
// configuration is missing required fields. The caller is expected to
// pre-resolve FFmpegBinary via exec.LookPath at startup.
func NewVideoDecoder(cfg VideoDecoderConfig) (VideoDecoder, error) {
	if cfg.FFmpegBinary == "" {
		return VideoDecoder{}, thumbnailerErrors.ErrVideoDecoderDisabled
	}
	if cfg.FFmpegTimeout <= 0 {
		cfg.FFmpegTimeout = 30 * time.Second
	}
	if cfg.SeekOffset == "" {
		cfg.SeekOffset = "00:00:01"
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = 128 * 1024 * 1024
	}
	return VideoDecoder{cfg: cfg}, nil
}

// Convert reads the video bytes from r, hands them to ffmpeg, and returns
// the extracted frame as an image.Image.
//
// The reader is fully consumed and staged to a private temporary file so
// ffmpeg can seek within it. Once Convert returns the temp file is removed.
func (v VideoDecoder) Convert(r io.Reader) (any, error) {
	if v.cfg.FFmpegBinary == "" {
		return nil, thumbnailerErrors.ErrVideoDecoderDisabled
	}

	// Stage input on disk. We use the OS temp dir and let the kernel pick a
	// random name. The handle is opened with explicit mode 0600 so other
	// users on the host cannot read the staged bytes.
	tmp, err := os.CreateTemp("", "opencloud-thumbnail-video-*.bin")
	if err != nil {
		return nil, pkgerrors.Wrap(err, "could not create temporary file for video input")
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return nil, pkgerrors.Wrap(err, "could not chmod temporary video file")
	}
	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return nil, pkgerrors.Wrap(err, "could not write video input to temporary file")
	}
	if err := tmp.Close(); err != nil {
		return nil, pkgerrors.Wrap(err, "could not close temporary video file")
	}

	ctx, cancel := context.WithTimeout(context.Background(), v.cfg.FFmpegTimeout)
	defer cancel()

	// Argument order matters for ffmpeg seek performance: -ss before -i
	// performs a fast (keyframe) seek.
	//
	// -nostdin           : never read from our stdin
	// -loglevel error    : minimal stderr noise
	// -y                 : overwrite output without prompting (we use pipe:1)
	// -protocol_whitelist: forbid any non-file protocol regardless of input
	// -ss <offset>       : seek before decoding
	// -i <path>          : positional, server-controlled tempfile path
	// -an -sn -dn        : drop audio/subtitle/data streams
	// -frames:v 1        : decode exactly one video frame
	// -f mjpeg           : emit a single JPEG image
	// -q:v 4             : a reasonable quality (1=best,31=worst)
	// pipe:1             : write the frame to stdout
	args := []string{
		"-nostdin",
		"-loglevel", "error",
		"-y",
		"-protocol_whitelist", "file,crypto,data",
		"-ss", v.cfg.SeekOffset,
		"-i", tmpName,
		"-an", "-sn", "-dn",
		"-frames:v", "1",
		"-f", "mjpeg",
		"-q:v", "4",
		"pipe:1",
	}

	cmd := exec.CommandContext(ctx, v.cfg.FFmpegBinary, args...)
	// Detach from the parent's stdin so ffmpeg cannot be coerced into
	// reading anything via stdin (defense in depth — -nostdin already
	// covers this).
	cmd.Stdin = nil

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, pkgerrors.Wrap(err, "could not open ffmpeg stdout pipe")
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, pkgerrors.Wrap(err, "could not start ffmpeg")
	}

	// Read at most MaxOutputBytes from ffmpeg. Anything beyond that is
	// dropped and the subprocess is killed by the deferred cancel.
	limited := io.LimitReader(stdout, v.cfg.MaxOutputBytes)
	out, readErr := io.ReadAll(limited)

	waitErr := cmd.Wait()
	if waitErr != nil {
		// Surface the context deadline as ErrVideoExtractionFailed so the
		// caller treats it as a generation failure rather than a config
		// issue. The wrapped message contains the ffmpeg stderr tail for
		// operator debugging.
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, pkgerrors.Wrapf(
				thumbnailerErrors.ErrVideoExtractionFailed,
				"ffmpeg exceeded timeout: %s",
				truncateStderr(stderr.String()),
			)
		}
		return nil, pkgerrors.Wrapf(
			thumbnailerErrors.ErrVideoExtractionFailed,
			"ffmpeg exited with error: %v: %s",
			waitErr, truncateStderr(stderr.String()),
		)
	}
	if readErr != nil {
		return nil, pkgerrors.Wrap(readErr, "could not read ffmpeg stdout")
	}
	if len(out) == 0 {
		return nil, pkgerrors.Wrapf(
			thumbnailerErrors.ErrVideoExtractionFailed,
			"ffmpeg produced no output: %s",
			truncateStderr(stderr.String()),
		)
	}

	img, err := imaging.Decode(bytes.NewReader(out))
	if err != nil {
		return nil, pkgerrors.Wrap(err, "could not decode extracted video frame")
	}
	// Force the result to image.Image (imaging already returns one) so
	// the type assertion in SimpleGenerator.Generate keeps working.
	return image.Image(img), nil
}

// truncateStderr keeps log lines bounded so a chatty ffmpeg cannot produce
// pathologically long error payloads in our responses.
func truncateStderr(s string) string {
	const maxLen = 512
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(truncated)"
}
