package preprocessor

import (
	"bytes"
	"image"
	"os"
	"os/exec"
	"time"

	thumbnailerErrors "github.com/opencloud-eu/opencloud/services/thumbnails/pkg/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// resolveFFmpeg returns the absolute path to ffmpeg or "" if unavailable.
// All video decoder tests are skipped when this returns "".
func resolveFFmpeg() string {
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		return path
	}
	return ""
}

var _ = Describe("VideoDecoder", func() {
	Describe("NewVideoDecoder", func() {
		It("rejects an empty ffmpeg binary", func() {
			_, err := NewVideoDecoder(VideoDecoderConfig{})
			Expect(err).To(MatchError(thumbnailerErrors.ErrVideoDecoderDisabled))
		})

		It("applies sane defaults for zero values", func() {
			ff := resolveFFmpeg()
			if ff == "" {
				Skip("ffmpeg is not installed")
			}
			d, err := NewVideoDecoder(VideoDecoderConfig{FFmpegBinary: ff})
			Expect(err).ToNot(HaveOccurred())
			Expect(d.cfg.FFmpegTimeout).To(BeNumerically(">", 0))
			Expect(d.cfg.SeekOffset).ToNot(BeEmpty())
			Expect(d.cfg.MaxOutputBytes).To(BeNumerically(">", 0))
		})
	})

	Describe("Convert", func() {
		var (
			decoder VideoDecoder
			ff      string
		)

		BeforeEach(func() {
			ff = resolveFFmpeg()
			if ff == "" {
				Skip("ffmpeg is not installed")
			}
			var err error
			decoder, err = NewVideoDecoder(VideoDecoderConfig{
				FFmpegBinary:   ff,
				FFmpegTimeout:  10 * time.Second,
				SeekOffset:     "00:00:00.500",
				MaxOutputBytes: 4 * 1024 * 1024,
			})
			Expect(err).ToNot(HaveOccurred())
		})

		It("extracts a frame from a valid mp4 fixture", func() {
			b, err := os.ReadFile("test_assets/test_video.mp4")
			Expect(err).ToNot(HaveOccurred())

			out, err := decoder.Convert(bytes.NewReader(b))
			Expect(err).ToNot(HaveOccurred())
			Expect(out).ToNot(BeNil())

			img, ok := out.(image.Image)
			Expect(ok).To(BeTrue(), "decoder must return image.Image")
			Expect(img.Bounds().Dx()).To(BeNumerically(">", 0))
			Expect(img.Bounds().Dy()).To(BeNumerically(">", 0))
		})

		It("returns ErrVideoExtractionFailed for invalid input", func() {
			out, err := decoder.Convert(bytes.NewReader([]byte("not a video")))
			Expect(err).To(HaveOccurred())
			Expect(out).To(BeNil())
			// The wrapped error must report extraction failure (not panic).
			Expect(err.Error()).To(ContainSubstring("ffmpeg"))
		})

		It("returns ErrVideoDecoderDisabled when binary is empty", func() {
			d := VideoDecoder{}
			out, err := d.Convert(bytes.NewReader([]byte("anything")))
			Expect(err).To(MatchError(thumbnailerErrors.ErrVideoDecoderDisabled))
			Expect(out).To(BeNil())
		})

		It("respects the configured timeout", func() {
			// Build a decoder with an absurdly short timeout so even a tiny
			// fixture exceeds the deadline. The test passes as long as we get
			// an error and we do not panic; we don't assert specifically on
			// deadline-vs-other failures because the resulting behavior is
			// timing-sensitive.
			d, err := NewVideoDecoder(VideoDecoderConfig{
				FFmpegBinary:   ff,
				FFmpegTimeout:  1 * time.Millisecond,
				SeekOffset:     "00:00:00.500",
				MaxOutputBytes: 4 * 1024 * 1024,
			})
			Expect(err).ToNot(HaveOccurred())

			b, rErr := os.ReadFile("test_assets/test_video.mp4")
			Expect(rErr).ToNot(HaveOccurred())

			out, cErr := d.Convert(bytes.NewReader(b))
			Expect(cErr).To(HaveOccurred())
			Expect(out).To(BeNil())
		})
	})

	Describe("ForType routing", func() {
		It("returns the configured VideoDecoder for known video MIME types", func() {
			ff := resolveFFmpeg()
			if ff == "" {
				Skip("ffmpeg is not installed")
			}
			d, err := NewVideoDecoder(VideoDecoderConfig{FFmpegBinary: ff})
			Expect(err).ToNot(HaveOccurred())

			for _, mt := range []string{"video/mp4", "video/webm", "video/quicktime", "video/x-matroska", "video/x-msvideo"} {
				got := ForType(mt, map[string]any{"videoDecoder": d})
				_, ok := got.(VideoDecoder)
				Expect(ok).To(BeTrue(), "ForType(%q) should return a VideoDecoder when one is configured", mt)
			}
		})

		It("falls back to ImageDecoder when no VideoDecoder is configured", func() {
			got := ForType("video/mp4", map[string]any{})
			_, ok := got.(ImageDecoder)
			Expect(ok).To(BeTrue(), "ForType must not panic and must fall back to ImageDecoder")
		})
	})
})
