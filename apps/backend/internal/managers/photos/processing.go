package photos

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"io"
	"math"

	"bikers.kz/backend/internal/models"
	"github.com/bep/imagemeta"
	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
	_ "golang.org/x/image/webp"
)

const (
	// MaxPhotoUploadBytes bounds the original body before metadata or pixel decoding.
	MaxPhotoUploadBytes = 12 << 20
	photoOutputLimit    = 4 << 20
	photoPixelLimit     = 32_000_000
	photoEdgeLimit      = 12000
	photoOutputEdge     = 1920
	photoMetadataBytes  = 64 << 20
	photoMetadataOps    = 64 << 10
)

func invalidPhoto() error {
	return models.Fail("invalid_photo", "The image is invalid or cannot be processed")
}
func photoTooLarge() error {
	return models.Fail("photo_too_large", "The image exceeds the photo size limits")
}

// processPhoto retains normalized pixels only. The caller owns the processing
// semaphore; cancellation does not preempt a codec already executing CPU work.
func processPhoto(ctx context.Context, input io.Reader) (models.ProcessedPhoto, error) {
	if err := ctx.Err(); err != nil {
		return models.ProcessedPhoto{}, err
	}
	if input == nil {
		return models.ProcessedPhoto{}, invalidPhoto()
	}
	data, err := io.ReadAll(io.LimitReader(photoReader{ctx: ctx, reader: input}, MaxPhotoUploadBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return models.ProcessedPhoto{}, ctx.Err()
		}
		return models.ProcessedPhoto{}, invalidPhoto()
	}
	if err := ctx.Err(); err != nil {
		return models.ProcessedPhoto{}, err
	}
	if len(data) > MaxPhotoUploadBytes {
		return models.ProcessedPhoto{}, photoTooLarge()
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		if hasPhotoSignature(data) {
			return models.ProcessedPhoto{}, invalidPhoto()
		}
		return models.ProcessedPhoto{}, models.Fail("unsupported_photo", "Use a JPEG, PNG, or WebP image; export HEIC as JPEG first")
	}
	var metadataFormat imagemeta.ImageFormat
	switch format {
	case "jpeg":
		metadataFormat = imagemeta.JPEG
	case "png":
		metadataFormat = imagemeta.PNG
	case "webp":
		metadataFormat = imagemeta.WebP
	default:
		return models.ProcessedPhoto{}, models.Fail("unsupported_photo", "Use a JPEG, PNG, or WebP image")
	}
	if config.Width < 1 || config.Height < 1 {
		return models.ProcessedPhoto{}, invalidPhoto()
	}
	if config.Width > photoEdgeLimit || config.Height > photoEdgeLimit || config.Width > photoPixelLimit/config.Height {
		return models.ProcessedPhoto{}, photoTooLarge()
	}
	if err := ctx.Err(); err != nil {
		return models.ProcessedPhoto{}, err
	}
	if err := checkStillPhotoContainer(data, format); err != nil {
		return models.ProcessedPhoto{}, err
	}
	orientation, err := readPhotoOrientation(ctx, bytes.NewReader(data), metadataFormat)
	if ctx.Err() != nil {
		return models.ProcessedPhoto{}, ctx.Err()
	}
	if err != nil {
		return models.ProcessedPhoto{}, invalidPhoto()
	}
	source, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if ctx.Err() != nil {
		return models.ProcessedPhoto{}, ctx.Err()
	}
	if err != nil || decodedFormat != format || source.Bounds().Dx() != config.Width || source.Bounds().Dy() != config.Height {
		return models.ProcessedPhoto{}, invalidPhoto()
	}
	width, height := config.Width, config.Height
	if orientation >= 5 {
		width, height = height, width
	}
	scale := math.Min(1, math.Min(float64(photoOutputEdge)/float64(width), float64(photoOutputEdge)/float64(height)))
	outWidth := max(1, int(math.Round(float64(width)*scale)))
	outHeight := max(1, int(math.Round(float64(height)*scale)))
	output := image.NewRGBA(image.Rect(0, 0, outWidth, outHeight))
	draw.Draw(output, output.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	// Apply EXIF orientation and resizing in one affine resampling pass rather
	// than allocating another full-resolution rotated image.
	transform := photoTransform(orientation, config.Width, config.Height)
	sourceBounds := source.Bounds()
	transform[2] -= transform[0]*float64(sourceBounds.Min.X) + transform[1]*float64(sourceBounds.Min.Y)
	transform[5] -= transform[3]*float64(sourceBounds.Min.X) + transform[4]*float64(sourceBounds.Min.Y)
	sx, sy := float64(outWidth)/float64(width), float64(outHeight)/float64(height)
	for i := 0; i < 3; i++ {
		transform[i] *= sx
		transform[i+3] *= sy
	}
	draw.CatmullRom.Transform(output, transform, source, sourceBounds, draw.Over, nil)
	if err := ctx.Err(); err != nil {
		return models.ProcessedPhoto{}, err
	}
	encoded := photoWriter{ctx: ctx}
	if err := jpeg.Encode(&encoded, output, &jpeg.Options{Quality: 85}); err != nil {
		if ctx.Err() != nil {
			return models.ProcessedPhoto{}, ctx.Err()
		}
		if encoded.tooLarge {
			return models.ProcessedPhoto{}, photoTooLarge()
		}
		return models.ProcessedPhoto{}, models.Fail("service_unavailable", "Photo processing is temporarily unavailable")
	}
	if err := ctx.Err(); err != nil {
		return models.ProcessedPhoto{}, err
	}
	return models.ProcessedPhoto{Bytes: encoded.Bytes(), Width: outWidth, Height: outHeight}, nil
}

func hasPhotoSignature(data []byte) bool {
	return bytes.HasPrefix(data, []byte{0xff, 0xd8}) || bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G'}) || (len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP")
}

// Decoders can expose only the first frame, and a one-frame animation still has
// animation semantics. Inspect bounded container chunks rather than frame count;
// pixel decoding and metadata parsing remain the established libraries' job.
func checkStillPhotoContainer(data []byte, format string) error {
	switch format {
	case "png":
		if len(data) < 8 {
			return invalidPhoto()
		}
		for offset := 8; offset < len(data); {
			if len(data)-offset < 12 {
				return invalidPhoto()
			}
			length := uint64(binary.BigEndian.Uint32(data[offset : offset+4]))
			if length > uint64(len(data)-offset-12) {
				return invalidPhoto()
			}
			kind := string(data[offset+4 : offset+8])
			if kind == "acTL" || kind == "fcTL" || kind == "fdAT" {
				return invalidPhoto()
			}
			offset += int(length) + 12
			if kind == "IEND" {
				if length != 0 || offset != len(data) {
					return invalidPhoto()
				}
				return nil
			}
		}
		return invalidPhoto()
	case "webp":
		if len(data) < 12 || uint64(binary.LittleEndian.Uint32(data[4:8]))+8 != uint64(len(data)) {
			return invalidPhoto()
		}
		for offset := 12; offset < len(data); {
			if len(data)-offset < 8 {
				return invalidPhoto()
			}
			length := uint64(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
			padded := length + (length & 1)
			if padded > uint64(len(data)-offset-8) {
				return invalidPhoto()
			}
			kind := string(data[offset : offset+4])
			if kind == "ANIM" || kind == "ANMF" {
				return invalidPhoto()
			}
			if kind == "VP8X" && (length != 10 || data[offset+8]&0x02 != 0) {
				return invalidPhoto()
			}
			offset += int(padded) + 8
		}
	}
	return nil
}

func readPhotoOrientation(ctx context.Context, reader io.ReadSeeker, format imagemeta.ImageFormat) (uint16, error) {
	orientation := uint16(1)
	bounded := &photoMetadataReader{ctx: ctx, reader: reader, bytesLeft: photoMetadataBytes, operationsLeft: photoMetadataOps}
	_, err := imagemeta.Decode(imagemeta.Options{
		R:           bounded,
		ImageFormat: format, Sources: imagemeta.EXIF,
		// Only the two-byte scalar IFD0 Orientation is needed. The size limit
		// also skips every four-byte IFD pointer and pointer array, preventing
		// repeated SubIFD traversal inside the library's copied EXIF buffer.
		LimitNumTags: 1024, LimitTagSize: 2,
		// A positive imagemeta timeout spawns a decoder goroutine and returns
		// without joining it. Keep this synchronous so the caller's semaphore
		// remains held until all metadata work and callbacks have stopped.
		Timeout: 0,
		ShouldHandleTag: func(tag imagemeta.TagInfo) bool {
			return tag.Source == imagemeta.EXIF && tag.Namespace == "IFD0" && tag.Tag == "Orientation"
		},
		HandleTag: func(tag imagemeta.TagInfo) error {
			value, ok := tag.Value.(uint16)
			if !ok || value < 1 || value > 8 {
				return invalidPhoto()
			}
			orientation = value
			return imagemeta.ErrStopWalking
		},
	})
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	if bounded.failure != nil {
		return 0, bounded.failure
	}
	return orientation, err
}

var errPhotoMetadataWork = errors.New("photo metadata work limit exceeded")

// These limits bound container reads/seeks, not the library's private EXIF
// buffer. LimitTagSize above separately prevents nested EXIF traversal.
type photoMetadataReader struct {
	ctx            context.Context
	reader         io.ReadSeeker
	bytesLeft      int
	operationsLeft int
	failure        error
}

func (r *photoMetadataReader) next() error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if r.operationsLeft <= 0 {
		r.failure = errPhotoMetadataWork
		return errPhotoMetadataWork
	}
	r.operationsLeft--
	return nil
}
func (r *photoMetadataReader) Read(p []byte) (int, error) {
	if err := r.next(); err != nil {
		return 0, err
	}
	if r.bytesLeft <= 0 {
		r.failure = errPhotoMetadataWork
		return 0, errPhotoMetadataWork
	}
	if len(p) > r.bytesLeft {
		p = p[:r.bytesLeft]
	}
	n, err := r.reader.Read(p)
	r.bytesLeft -= n
	return n, err
}
func (r *photoMetadataReader) Seek(offset int64, whence int) (int64, error) {
	if err := r.next(); err != nil {
		return 0, err
	}
	return r.reader.Seek(offset, whence)
}

// Transforms map source pixel edges into correctly oriented destination edges.
func photoTransform(orientation uint16, width, height int) f64.Aff3 {
	w, h := float64(width), float64(height)
	switch orientation {
	case 2:
		return f64.Aff3{-1, 0, w, 0, 1, 0}
	case 3:
		return f64.Aff3{-1, 0, w, 0, -1, h}
	case 4:
		return f64.Aff3{1, 0, 0, 0, -1, h}
	case 5:
		return f64.Aff3{0, 1, 0, 1, 0, 0}
	case 6:
		return f64.Aff3{0, -1, h, 1, 0, 0}
	case 7:
		return f64.Aff3{0, -1, h, -1, 0, w}
	case 8:
		return f64.Aff3{0, 1, 0, -1, 0, w}
	default:
		return f64.Aff3{1, 0, 0, 0, 1, 0}
	}
}

type photoReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r photoReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

type photoWriter struct {
	bytes.Buffer
	ctx      context.Context
	tooLarge bool
}

func (w *photoWriter) Write(data []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if len(data) > photoOutputLimit-w.Len() {
		w.tooLarge = true
		return 0, photoTooLarge()
	}
	return w.Buffer.Write(data)
}
