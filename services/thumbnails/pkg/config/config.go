// Package config contains the configuration for the opencloud-thumbnails service
package config

import (
	"context"
	"time"

	"github.com/opencloud-eu/opencloud/pkg/shared"
	"go-micro.dev/v4/client"
)

// Config combines all available configuration parts.
type Config struct {
	Commons *shared.Commons `yaml:"-"` // don't use this directly as configuration for a service

	Service Service `yaml:"-"`

	LogLevel string `yaml:"loglevel" env:"OC_LOG_LEVEL;THUMBNAILS_LOG_LEVEL" desc:"The log level. Valid values are: 'panic', 'fatal', 'error', 'warn', 'info', 'debug', 'trace'." introductionVersion:"1.0.0"`
	Debug    Debug  `yaml:"debug"`

	GRPC GRPCConfig `yaml:"grpc"`
	HTTP HTTP       `yaml:"http"`

	GRPCClientTLS *shared.GRPCClientTLS `yaml:"grpc_client_tls"`
	GrpcClient    client.Client         `yaml:"-"`

	Thumbnail Thumbnail `yaml:"thumbnail"`

	Context context.Context `yaml:"-"`
}

// FileSystemStorage defines the available filesystem storage configuration.
type FileSystemStorage struct {
	RootDirectory string `yaml:"root_directory" env:"THUMBNAILS_FILESYSTEMSTORAGE_ROOT" desc:"The directory where the filesystem storage will store the thumbnails. If not defined, the root directory derives from $OC_BASE_DATA_PATH/thumbnails." introductionVersion:"1.0.0"`
}

// Thumbnail defines the available thumbnail related configuration.
type Thumbnail struct {
	Resolutions           []string          `yaml:"resolutions" env:"THUMBNAILS_RESOLUTIONS" desc:"The supported list of target resolutions in the format WidthxHeight like 32x32. You can define any resolution as required. See the Environment Variable Types description for more details." introductionVersion:"1.0.0"`
	FileSystemStorage     FileSystemStorage `yaml:"filesystem_storage"`
	WebdavAllowInsecure   bool              `yaml:"webdav_allow_insecure" env:"OC_INSECURE;THUMBNAILS_WEBDAVSOURCE_INSECURE" desc:"Ignore untrusted SSL certificates when connecting to the webdav source." introductionVersion:"1.0.0"`
	CS3AllowInsecure      bool              `yaml:"cs3_allow_insecure" env:"OC_INSECURE;THUMBNAILS_CS3SOURCE_INSECURE" desc:"Ignore untrusted SSL certificates when connecting to the CS3 source." introductionVersion:"1.0.0"`
	RevaGateway           string            `yaml:"reva_gateway" env:"OC_REVA_GATEWAY" desc:"CS3 gateway used to look up user metadata" introductionVersion:"1.0.0"`
	FontMapFile           string            `yaml:"font_map_file" env:"THUMBNAILS_TXT_FONTMAP_FILE" desc:"The path to a font file for txt thumbnails." introductionVersion:"1.0.0"`
	TransferSecret        string            `yaml:"transfer_secret" env:"THUMBNAILS_TRANSFER_TOKEN" desc:"The secret to sign JWT to download the actual thumbnail file." introductionVersion:"1.0.0"`
	DataEndpoint          string            `yaml:"data_endpoint" env:"THUMBNAILS_DATA_ENDPOINT" desc:"The HTTP endpoint where the actual thumbnail file can be downloaded." introductionVersion:"1.0.0"`
	MaxInputWidth         int               `yaml:"max_input_width" env:"THUMBNAILS_MAX_INPUT_WIDTH" desc:"The maximum width of an input image which is being processed." introductionVersion:"1.0.0"`
	MaxInputHeight        int               `yaml:"max_input_height" env:"THUMBNAILS_MAX_INPUT_HEIGHT" desc:"The maximum height of an input image which is being processed." introductionVersion:"1.0.0"`
	MaxInputImageFileSize string            `yaml:"max_input_image_file_size" env:"THUMBNAILS_MAX_INPUT_IMAGE_FILE_SIZE" desc:"The maximum file size of an input image which is being processed. Usable common abbreviations: [KB, KiB, MB, MiB, GB, GiB, TB, TiB, PB, PiB, EB, EiB], example: 2GB." introductionVersion:"1.0.0"`
	Video                 Video             `yaml:"video"`
}

// Video defines the available video thumbnail related configuration.
type Video struct {
	Enabled          bool          `yaml:"enabled" env:"THUMBNAILS_VIDEO_ENABLED" desc:"Enable video thumbnail generation. Requires the ffmpeg binary to be available at startup. If ffmpeg is not found, video thumbnails stay disabled regardless of this flag." introductionVersion:"7.0.0"`
	FFmpegBinary     string        `yaml:"ffmpeg_binary" env:"THUMBNAILS_VIDEO_FFMPEG_BINARY" desc:"The path to (or name of) the ffmpeg binary used for extracting video frames. If only a name is given, the binary is resolved via PATH at startup." introductionVersion:"7.0.0"`
	FFmpegTimeout    time.Duration `yaml:"ffmpeg_timeout" env:"THUMBNAILS_VIDEO_FFMPEG_TIMEOUT" desc:"Maximum wall-clock duration of the ffmpeg subprocess per request. The subprocess is killed when this elapses." introductionVersion:"7.0.0"`
	MaxInputFileSize string        `yaml:"max_input_file_size" env:"THUMBNAILS_VIDEO_MAX_INPUT_FILE_SIZE" desc:"The maximum file size of an input video which is being processed. Files larger than this are rejected before any frame extraction is attempted. Usable common abbreviations: [KB, KiB, MB, MiB, GB, GiB, TB, TiB]. Example: 512MB." introductionVersion:"7.0.0"`
	SeekOffset       string        `yaml:"seek_offset" env:"THUMBNAILS_VIDEO_SEEK_OFFSET" desc:"The offset (HH:MM:SS or ffmpeg-compatible duration) where the frame is extracted. Server-controlled; never user-supplied." introductionVersion:"7.0.0"`
	MimeTypes        []string      `yaml:"mimetypes" env:"THUMBNAILS_VIDEO_MIMETYPES" desc:"Comma separated list of video MIME types to enable previews for. Defaults to mp4, webm, quicktime/mov, mkv and avi. See the Environment Variable Types description for more details." introductionVersion:"7.0.0"`
	MaxOutputBytes   int64         `yaml:"max_output_bytes" env:"THUMBNAILS_VIDEO_MAX_OUTPUT_BYTES" desc:"Hard cap on the number of bytes read from ffmpeg's stdout per request. Defends against pathological inputs producing very large frames." introductionVersion:"7.0.0"`
}
