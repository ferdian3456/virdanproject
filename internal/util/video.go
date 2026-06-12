package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
)

var allowedVideoMIMEs = map[string]bool{
	"video/mp4":       true,
	"video/quicktime": true,
	"video/webm":      true,
}

var validVideoExts = map[string]bool{
	".mp4": true, ".mov": true, ".webm": true,
}

// ffprobeOutput holds the JSON structure returned by ffprobe.
type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeSideData struct {
	SideDataType string  `json:"side_data_type"`
	Rotation     float64 `json:"rotation"`
}

type ffprobeStream struct {
	CodecType    string            `json:"codec_type"`
	Width        int               `json:"width"`
	Height       int               `json:"height"`
	Tags         map[string]string `json:"tags"`
	SideDataList []ffprobeSideData `json:"side_data_list"`
}

type ffprobeFormat struct {
	Duration string `json:"duration"`
}

// ValidateVideoFile validates a multipart file header for video:
// checks size, file extension, and MIME type via content sniffing.
func ValidateVideoFile(ctx context.Context, fileHeader *multipart.FileHeader, fieldName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if fileHeader.Size > constant.MAX_VIDEO_SIZE {
		return &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "video size exceeded " + strconv.FormatInt(constant.MAX_VIDEO_SIZE/(1024*1024), 10) + "MB limit",
			Param:   fieldName,
		}
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !validVideoExts[ext] {
		return &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "invalid video extension: " + ext + ". Allowed: .mp4, .mov, .webm",
			Param:   fieldName,
		}
	}

	f, err := fileHeader.Open()
	if err != nil {
		return &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: fieldName + " could not be opened",
			Param:   fieldName,
		}
	}
	defer func() { _ = f.Close() }()

	var sniff [512]byte
	n, _ := f.Read(sniff[:])
	detected := http.DetectContentType(sniff[:n])

	// http.DetectContentType returns "application/octet-stream" for many video formats,
	// so we also trust the extension if sniffing is inconclusive.
	if detected != "application/octet-stream" && !allowedVideoMIMEs[detected] {
		return &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "invalid video type: " + detected,
			Param:   fieldName,
		}
	}

	return nil
}

// ProbeVideoMetadata uses ffprobe to extract duration, width, and height from a video file.
// Returns duration in seconds, width, height.
func ProbeVideoMetadata(ctx context.Context, filePath string) (int, int, int, error) {
	if err := ctx.Err(); err != nil {
		return 0, 0, 0, err
	}

	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	}

	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, 0, 0, fmt.Errorf("ffprobe failed: %w: %s", err, stderr.String())
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(stdout.Bytes(), &probe); err != nil {
		return 0, 0, 0, fmt.Errorf("ffprobe output parse failed: %w", err)
	}

	durationF, err := strconv.ParseFloat(probe.Format.Duration, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("ffprobe duration parse failed: %w", err)
	}
	duration := int(durationF)

	var width, height int
	for _, s := range probe.Streams {
		if s.CodecType == "video" {
			width = s.Width
			height = s.Height

			rotated := false
			// Check legacy tags.rotate (older MP4 files)
			if s.Tags != nil {
				if rotStr, ok := s.Tags["rotate"]; ok {
					if rotStr == "90" || rotStr == "270" || rotStr == "-90" || rotStr == "-270" {
						width, height = height, width
						rotated = true
					}
				}
			}
			// Check side_data_list Display Matrix (modern Android/iOS videos)
			if !rotated {
				for _, sd := range s.SideDataList {
					if sd.SideDataType == "Display Matrix" {
						rot := sd.Rotation
						if rot == 90 || rot == -90 || rot == 270 || rot == -270 {
							width, height = height, width
						}
						break
					}
				}
			}
			break
		}
	}
	if width == 0 || height == 0 {
		return 0, 0, 0, fmt.Errorf("ffprobe: no video stream found in %s", filePath)
	}

	return duration, width, height, nil
}

// GenerateVideoThumbnail uses ffmpeg to extract the first frame of a video
// and converts it to WebP format. Returns the raw WebP bytes.
func GenerateVideoThumbnail(ctx context.Context, filePath string, quality int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	args := []string{
		"-i", filePath,
		"-vframes", "1",
		"-q:v", strconv.Itoa(quality),
		"-f", "webp",
		"-y",
		"pipe:1",
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg thumbnail generation failed: %w: %s", err, stderr.String())
	}

	if stdout.Len() == 0 {
		return nil, fmt.Errorf("ffmpeg produced empty thumbnail for %s", filePath)
	}

	return stdout.Bytes(), nil
}

// SaveMultipartToTemp saves a multipart file to a temporary path and returns
// the written file path and its io.Closer. Caller must close and remove the file.
func SaveMultipartToTemp(fileHeader *multipart.FileHeader, dir, prefix string) (string, io.Closer, error) {
	src, err := fileHeader.Open()
	if err != nil {
		return "", nil, err
	}

	ext := filepath.Ext(fileHeader.Filename)
	tmpPath := filepath.Join(dir, prefix+ext)

	f, err := os.Create(tmpPath)
	if err != nil {
		_ = src.Close()
		return "", nil, err
	}

	if _, err := io.Copy(f, src); err != nil {
		_ = f.Close()
		_ = src.Close()
		return "", nil, err
	}

	_ = src.Close()
	return tmpPath, f, nil
}
