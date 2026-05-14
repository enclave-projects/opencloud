package defaults

import (
	"path"
	"strings"
	"time"

	"github.com/opencloud-eu/opencloud/pkg/config/defaults"
	"github.com/opencloud-eu/opencloud/pkg/shared"
	"github.com/opencloud-eu/opencloud/pkg/structs"
	"github.com/opencloud-eu/opencloud/services/thumbnails/pkg/config"
)

// DefaultVideoMimeTypes returns the default list of video MIME types for which
// the thumbnail service generates previews when ffmpeg is available.
//
// Kept exported so the runtime startup hook can reuse the same list when
// registering MIME types with the thumbnail package.
func DefaultVideoMimeTypes() []string {
	return []string{
		"video/mp4",
		"video/webm",
		"video/quicktime",
		"video/x-matroska",
		"video/x-msvideo",
	}
}

// FullDefaultConfig returns a fully initialized default configuration
func FullDefaultConfig() *config.Config {
	cfg := DefaultConfig()
	EnsureDefaults(cfg)
	Sanitize(cfg)
	return cfg
}

// DefaultConfig returns a basic default configuration
func DefaultConfig() *config.Config {
	return &config.Config{
		Debug: config.Debug{
			Addr:   "127.0.0.1:9189",
			Token:  "",
			Pprof:  false,
			Zpages: false,
		},
		GRPC: config.GRPCConfig{
			Addr:                  "127.0.0.1:9185",
			Namespace:             "eu.opencloud.api",
			MaxConcurrentRequests: 0,
		},
		HTTP: config.HTTP{
			Addr:      "127.0.0.1:9186",
			Root:      "/thumbnails",
			Namespace: "eu.opencloud.web",
			CORS: config.CORS{
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
				AllowedHeaders:   []string{"Authorization", "Origin", "Content-Type", "Accept", "X-Requested-With", "X-Request-Id", "Cache-Control"},
				AllowCredentials: true,
			},
		},
		Service: config.Service{
			Name: "thumbnails",
		},
		Thumbnail: config.Thumbnail{
			Resolutions: []string{"16x16", "32x32", "64x64", "128x128", "1080x1920", "1920x1080", "2160x3840", "3840x2160", "4320x7680", "7680x4320"},
			FileSystemStorage: config.FileSystemStorage{
				RootDirectory: path.Join(defaults.BaseDataPath(), "thumbnails"),
			},
			WebdavAllowInsecure:   false,
			RevaGateway:           shared.DefaultRevaConfig().Address,
			CS3AllowInsecure:      false,
			DataEndpoint:          "http://127.0.0.1:9186/thumbnails/data",
			MaxInputWidth:         7680,
			MaxInputHeight:        7680,
			MaxInputImageFileSize: "50MB",
			Video: config.Video{
				Enabled:          true,
				FFmpegBinary:     "ffmpeg",
				FFmpegTimeout:    30 * time.Second,
				MaxInputFileSize: "512MB",
				SeekOffset:       "00:00:01",
				MimeTypes:        DefaultVideoMimeTypes(),
				MaxOutputBytes:   128 * 1024 * 1024,
			},
		},
	}
}

// EnsureDefaults adds default values to the configuration if they are not set yet
func EnsureDefaults(cfg *config.Config) {
	if cfg.LogLevel == "" {
		cfg.LogLevel = "error"
	}

	if cfg.GRPCClientTLS == nil && cfg.Commons != nil {
		cfg.GRPCClientTLS = structs.CopyOrZeroValue(cfg.Commons.GRPCClientTLS)
	}
	if cfg.GRPC.TLS == nil && cfg.Commons != nil {
		cfg.GRPC.TLS = structs.CopyOrZeroValue(cfg.Commons.GRPCServiceTLS)
	}

	if cfg.Commons != nil {
		cfg.HTTP.TLS = cfg.Commons.HTTPServiceTLS
	}
}

// Sanitize sanitized the configuration
func Sanitize(cfg *config.Config) {
	// nothing to sanitize here atm
	if len(cfg.Thumbnail.Resolutions) == 1 && strings.Contains(cfg.Thumbnail.Resolutions[0], ",") {
		cfg.Thumbnail.Resolutions = strings.Split(cfg.Thumbnail.Resolutions[0], ",")
	}
	// Allow comma separated MIME types when supplied via a single env variable.
	if len(cfg.Thumbnail.Video.MimeTypes) == 1 && strings.Contains(cfg.Thumbnail.Video.MimeTypes[0], ",") {
		cfg.Thumbnail.Video.MimeTypes = strings.Split(cfg.Thumbnail.Video.MimeTypes[0], ",")
	}
	for i, mt := range cfg.Thumbnail.Video.MimeTypes {
		cfg.Thumbnail.Video.MimeTypes[i] = strings.TrimSpace(mt)
	}
}
