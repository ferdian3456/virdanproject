package util

import (
	"bytes"
	"fmt"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/h2non/bimg"
)

var AllowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/jpg":  true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

func ValidateImage(fileHeader *multipart.FileHeader, fieldName string) (*bytes.Reader, int64, error) {
	if fileHeader.Size > constant.MAX_FILE_SIZE {
		return nil, 0, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: fmt.Sprintf("Image size exceeded %dMB limit", constant.MAX_FILE_SIZE/(1024*1024)),
			Param:   fieldName,
		}
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if !AllowedImageTypes[contentType] {
		return nil, 0, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: fmt.Sprintf("Invalid file type: %s. allowed types: jpeg, jpg, png, gif, webp", contentType),
			Param:   fieldName,
		}
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	validExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	if !validExts[ext] {
		return nil, 0, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: fmt.Sprintf("Invalid file extension: %s", ext),
			Param:   fieldName,
		}
	}

	webBuf, err := ConvertToWebP(fileHeader, 75, 512, 512)
	if err != nil {
		return nil, 0, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Failed to process image. File may be corrupted or not a valid image",
			Param:   fieldName,
		}
	}

	webpSize := int64(webBuf.Len())

	return bytes.NewReader(webBuf.Bytes()), webpSize, nil
}

func ConvertToWebP(file *multipart.FileHeader, quality int, maxW int, maxH int) (*bytes.Buffer, error) {
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	// Read file into buffer
	buffer := new(bytes.Buffer)
	_, err = buffer.ReadFrom(src)
	if err != nil {
		return nil, err
	}

	output, err := bimg.NewImage(buffer.Bytes()).Process(bimg.Options{
		Width:   maxW,
		Height:  maxH,
		Quality: quality,
		Type:    bimg.WEBP,
		Crop:    true,
		Embed:   false, // Maintains aspect ratio
		Force:   true,  // Dont force exact dimension
	})

	if err != nil {
		return nil, err
	}

	return bytes.NewBuffer(output), nil
}
