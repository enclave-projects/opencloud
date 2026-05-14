Enhancement: Add video thumbnail support

The thumbnails service can now generate previews for video files. By default
mp4, webm, mov/quicktime, mkv/matroska and avi inputs are supported. A frame
is extracted at startup-configured offset (default `00:00:01`) by invoking the
`ffmpeg` binary, fed into the existing image pipeline and cached by file
checksum so only the first request per file pays the extraction cost.

The feature requires `ffmpeg` to be available at service startup. The official
container image ships `ffmpeg`. When the binary is not found the video pipeline
stays disabled, no MIME types are registered and responses are byte-identical
to previous releases.

All ffmpeg invocations are sandboxed: direct `exec.CommandContext` (no shell),
random-named tempfile staged with mode `0600`, server-controlled arguments,
`-protocol_whitelist file,crypto,data` to block remote IO, `-frames:v 1 -an
-sn -dn` to do the minimum work, `io.LimitReader` on stdout and a wall-clock
context timeout. New `THUMBNAILS_VIDEO_*` environment variables tune the
behavior; see `services/thumbnails/README.md`.

https://github.com/opencloud-eu/opencloud/pull/0
