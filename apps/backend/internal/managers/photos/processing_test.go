package photos

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"sync"
	"testing"
	"time"

	"bikers.kz/backend/internal/models"
	"github.com/bep/imagemeta"
)

func TestProcessBikePhotoOrientsRealEXIFAndStripsGPS(t *testing.T) {
	input := photoJPEG(t, quadrantPhoto())
	input = withPhotoEXIF(input, 6)
	tags := photoTags(t, input, imagemeta.JPEG)
	latitude, longitude, err := tags.GetLatLong()
	if err != nil || math.Abs(latitude-43.25) > 0.00001 || math.Abs(longitude-76.95) > 0.00001 {
		t.Fatalf("fixture must contain real GPS metadata: %v,%v %v", latitude, longitude, err)
	}
	if got := tags.EXIF()["Orientation"].Value; got != uint16(6) {
		t.Fatalf("fixture orientation = %v", got)
	}
	photo, err := processPhoto(context.Background(), bytes.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	output := decodedPhoto(t, photo, 80, 120)
	assertPhotoCorners(t, output, [4]color.RGBA{photoBlue, photoRed, photoYellow, photoGreen})
	if bytes.Contains(photo.Bytes, []byte("Exif\x00\x00")) {
		t.Fatal("output retained an EXIF payload")
	}
	outputTags := photoTags(t, photo.Bytes, imagemeta.JPEG)
	if len(outputTags.EXIF()) != 0 {
		t.Fatal("output retained EXIF/GPS tags")
	}
}

func TestProcessBikePhotoEXIFOrientations(t *testing.T) {
	input := photoJPEG(t, quadrantPhoto())
	cases := []struct {
		orientation   uint16
		width, height int
		corners       [4]color.RGBA
	}{
		{1, 120, 80, [4]color.RGBA{photoRed, photoGreen, photoBlue, photoYellow}},
		{2, 120, 80, [4]color.RGBA{photoGreen, photoRed, photoYellow, photoBlue}},
		{3, 120, 80, [4]color.RGBA{photoYellow, photoBlue, photoGreen, photoRed}},
		{4, 120, 80, [4]color.RGBA{photoBlue, photoYellow, photoRed, photoGreen}},
		{5, 80, 120, [4]color.RGBA{photoRed, photoBlue, photoGreen, photoYellow}},
		{6, 80, 120, [4]color.RGBA{photoBlue, photoRed, photoYellow, photoGreen}},
		{7, 80, 120, [4]color.RGBA{photoYellow, photoGreen, photoBlue, photoRed}},
		{8, 80, 120, [4]color.RGBA{photoGreen, photoYellow, photoRed, photoBlue}},
	}
	for _, test := range cases {
		t.Run(string(rune('0'+test.orientation)), func(t *testing.T) {
			photo, err := processPhoto(context.Background(), bytes.NewReader(withPhotoEXIF(input, test.orientation)))
			if err != nil {
				t.Fatal(err)
			}
			assertPhotoCorners(t, decodedPhoto(t, photo, test.width, test.height), test.corners)
		})
	}
	for _, orientation := range []uint16{0, 9} {
		_, err := processPhoto(context.Background(), bytes.NewReader(withPhotoEXIF(input, orientation)))
		assertPhotoFault(t, err, "invalid_photo")
	}
}

func TestProcessBikePhotoFlattensAlphaOnWhite(t *testing.T) {
	input := image.NewNRGBA(image.Rect(0, 0, 120, 80))
	draw.Draw(input, image.Rect(60, 0, 120, 80), image.NewUniform(color.NRGBA{R: 255, A: 128}), image.Point{}, draw.Src)
	photo, err := processPhoto(context.Background(), bytes.NewReader(photoPNG(t, input)))
	if err != nil {
		t.Fatal(err)
	}
	output := decodedPhoto(t, photo, 120, 80)
	assertPhotoColor(t, output.At(30, 40), color.RGBA{255, 255, 255, 255})
	assertPhotoColor(t, output.At(90, 40), color.RGBA{255, 127, 127, 255})
}

func TestProcessBikePhotoBoundsOutputWithoutUpscaling(t *testing.T) {
	for _, size := range []image.Point{{4000, 2000}, {5, 9}} {
		input := image.NewRGBA(image.Rectangle{Max: size})
		draw.Draw(input, input.Bounds(), image.NewUniform(photoBlue), image.Point{}, draw.Src)
		photo, err := processPhoto(context.Background(), bytes.NewReader(photoPNG(t, input)))
		if err != nil {
			t.Fatal(err)
		}
		want := size
		if size.X == 4000 {
			want = image.Pt(1920, 960)
		}
		output := decodedPhoto(t, photo, want.X, want.Y)
		assertPhotoColor(t, output.At(want.X/2, want.Y/2), photoBlue)
	}
}

func TestProcessBikePhotoDecodesOfficialWebPFixture(t *testing.T) {
	input := officialPhotoWebP(t)
	photo, err := processPhoto(context.Background(), bytes.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	output := decodedPhoto(t, photo, 75, 100)
	source, _, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	assertPhotoColor(t, output.At(30, 40), color.RGBAModel.Convert(source.At(30, 40)).(color.RGBA))
}

func TestProcessBikePhotoOrientsPNGAndWebPEXIF(t *testing.T) {
	stillPNG := photoPNG(t, quadrantPhoto())
	withPNGEXIF := append(bytes.Clone(stillPNG[:33]), photoPNGChunk("eXIf", photoEXIFTIFF(6))...)
	withPNGEXIF = append(withPNGEXIF, stillPNG[33:]...)
	photo, err := processPhoto(context.Background(), bytes.NewReader(withPNGEXIF))
	if err != nil {
		t.Fatal(err)
	}
	assertPhotoCorners(t, decodedPhoto(t, photo, 80, 120), [4]color.RGBA{photoBlue, photoRed, photoYellow, photoGreen})
	tags := photoTags(t, photo.Bytes, imagemeta.JPEG)
	if len(tags.EXIF()) != 0 {
		t.Fatal("PNG metadata survived JPEG encoding")
	}

	// The extended WebP header declares EXIF and the exact canvas dimensions.
	stillWebP := officialPhotoWebP(t)
	withWebPEXIF := append(bytes.Clone(stillWebP[:12]), photoWebPChunk("VP8X", []byte{0x08, 0, 0, 0, 74, 0, 0, 99, 0, 0})...)
	withWebPEXIF = append(withWebPEXIF, stillWebP[12:]...)
	withWebPEXIF = append(withWebPEXIF, photoWebPChunk("EXIF", photoEXIFTIFF(6))...)
	binary.LittleEndian.PutUint32(withWebPEXIF[4:], uint32(len(withWebPEXIF)-8))
	photo, err = processPhoto(context.Background(), bytes.NewReader(withWebPEXIF))
	if err != nil {
		t.Fatal(err)
	}
	decodedPhoto(t, photo, 100, 75)
	tags = photoTags(t, photo.Bytes, imagemeta.JPEG)
	if len(tags.EXIF()) != 0 {
		t.Fatal("WebP metadata survived JPEG encoding")
	}
}

func TestProcessBikePhotoRejectsAnimationContainers(t *testing.T) {
	stillPNG := photoPNG(t, quadrantPhoto())
	stillWebP := officialPhotoWebP(t)
	for _, test := range []struct {
		name string
		data []byte
	}{
		{"one-frame APNG", photoAPNG(stillPNG, 1)},
		{"multiple-frame APNG", photoAPNG(stillPNG, 2)},
		{"WebP animation flag", photoAnimatedWebP(stillWebP, "VP8X")},
		{"WebP animation control", photoAnimatedWebP(stillWebP, "ANIM")},
		{"WebP animation frame", photoAnimatedWebP(stillWebP, "ANMF")},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := processPhoto(context.Background(), bytes.NewReader(test.data))
			assertPhotoFault(t, err, "invalid_photo")
		})
	}
}

func TestProcessBikePhotoRejectsInvalidOrExcessiveInputs(t *testing.T) {
	validJPEG := photoJPEG(t, quadrantPhoto())
	validPNG := photoPNG(t, quadrantPhoto())
	validWebP := officialPhotoWebP(t)
	badCRC := bytes.Clone(validPNG)
	badCRC[len(badCRC)-1] ^= 0x80
	for _, test := range []struct {
		name, code string
		data       []byte
	}{
		{"empty", "unsupported_photo", nil},
		{"HEIC", "unsupported_photo", []byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00mif1heic")},
		{"AVIF", "unsupported_photo", []byte("\x00\x00\x00\x18ftypavif\x00\x00\x00\x00avifmif1")},
		{"GIF", "unsupported_photo", []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00")},
		{"SVG", "unsupported_photo", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)},
		{"truncated JPEG", "invalid_photo", validJPEG[:len(validJPEG)/2]},
		{"truncated PNG", "invalid_photo", validPNG[:len(validPNG)-12]},
		{"invalid PNG checksum", "invalid_photo", badCRC},
		{"truncated WebP", "invalid_photo", validWebP[:len(validWebP)-2]},
		{"oversized edge header", "photo_too_large", photoPNGHeader(12001, 1)},
		{"oversized pixel header", "photo_too_large", photoPNGHeader(8000, 4001)},
		{"zero width header", "invalid_photo", photoPNGHeader(0, 10)},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := processPhoto(context.Background(), bytes.NewReader(test.data))
			assertPhotoFault(t, err, test.code)
		})
	}
	_, err := processPhoto(context.Background(), nil)
	assertPhotoFault(t, err, "invalid_photo")
	_, err = processPhoto(context.Background(), photoFailingReader{})
	assertPhotoFault(t, err, "invalid_photo")
}

func TestProcessBikePhotoBoundsSourceReads(t *testing.T) {
	reader := &photoCountingReader{}
	_, err := processPhoto(context.Background(), reader)
	assertPhotoFault(t, err, "photo_too_large")
	if reader.bytes != (12<<20)+1 {
		t.Fatalf("read %d bytes from oversized body", reader.bytes)
	}
}

func TestProcessBikePhotoHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader := &photoCountingReader{}
	_, err := processPhoto(ctx, reader)
	if !errors.Is(err, context.Canceled) || reader.bytes != 0 {
		t.Fatalf("canceled before read: %v, bytes=%d", err, reader.bytes)
	}
	ctx, cancel = context.WithCancel(context.Background())
	_, err = processPhoto(ctx, photoCancelReader{cancel: cancel, data: photoPNG(t, quadrantPhoto())})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled during read: %v", err)
	}
}

func TestPhotoMetadataCancellationWaitsUntilDecoderStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := &photoBlockedMetadataReader{Reader: bytes.NewReader(withPhotoEXIF(photoJPEG(t, quadrantPhoto()), 6)), entered: make(chan struct{}), release: make(chan struct{})}
	release := sync.OnceFunc(func() { close(reader.release) })
	defer release()
	done := make(chan error, 1)
	go func() { _, err := readPhotoOrientation(ctx, reader, imagemeta.JPEG); done <- err }()
	select {
	case <-reader.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("metadata decoder did not start")
	}
	cancel()
	var result error
	returnedEarly := false
	// The former 500ms library timeout returned and pooled the reader while
	// this decode was still blocked. Cancellation must not release ownership
	// of an active decoder, even after that interval has passed.
	select {
	case result = <-done:
		returnedEarly = true
	case <-time.After(650 * time.Millisecond):
	}
	release()
	if !returnedEarly {
		select {
		case result = <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("metadata decoder did not stop after reader release")
		}
	}
	if returnedEarly {
		t.Fatal("metadata call returned while its decoder was still active")
	}
	if !errors.Is(result, context.Canceled) {
		t.Fatalf("canceled metadata decode returned %v", result)
	}
}

func TestProcessBikePhotoIgnoresThumbnailOrientationAndIFDChain(t *testing.T) {
	// IFD0 has no orientation; IFD1 claims orientation6 and points back to
	// itself. Thumbnail orientation and further IFD links do not affect pixels.
	tiff := make([]byte, 32)
	copy(tiff, "II")
	binary.LittleEndian.PutUint16(tiff[2:], 42)
	binary.LittleEndian.PutUint32(tiff[4:], 8)
	binary.LittleEndian.PutUint32(tiff[10:], 14)
	binary.LittleEndian.PutUint16(tiff[14:], 1)
	putPhotoIFD(tiff[16:], 0x0112, 3, 1, 6)
	binary.LittleEndian.PutUint32(tiff[28:], 14)
	photo, err := processPhoto(context.Background(), bytes.NewReader(photoPNGWithEXIF(t, tiff)))
	if err != nil {
		t.Fatal(err)
	}
	assertPhotoCorners(t, decodedPhoto(t, photo, 120, 80), [4]color.RGBA{photoRed, photoGreen, photoBlue, photoYellow})
}

func TestProcessBikePhotoSkipsAmplifiedSubIFDMetadata(t *testing.T) {
	// A SubIFD pointer array repeats one 65535-entry table 256 times. Each tag
	// is oversized, bypassing imagemeta's handled-tag counter. A final invalid
	// nested tag makes traversal fail observably. Only scalar IFD0 orientation
	// is relevant; the irrelevant pointer arrays must never be traversed.
	const pointerCount = 256
	const childCount = 65535
	const arrayOffset = 38
	const childOffset = arrayOffset + pointerCount*4
	tiff := make([]byte, childOffset+2+childCount*12+4)
	copy(tiff, "II")
	binary.LittleEndian.PutUint16(tiff[2:], 42)
	binary.LittleEndian.PutUint32(tiff[4:], 8)
	binary.LittleEndian.PutUint16(tiff[8:], 2)
	putPhotoIFD(tiff[10:], 0x014a, 4, pointerCount, arrayOffset)
	putPhotoIFD(tiff[22:], 0x0112, 3, 1, 6)
	for i := 0; i < pointerCount; i++ {
		binary.LittleEndian.PutUint32(tiff[arrayOffset+i*4:], childOffset)
	}
	binary.LittleEndian.PutUint16(tiff[childOffset:], childCount)
	for i := 0; i < childCount; i++ {
		putPhotoIFD(tiff[childOffset+2+i*12:], 0x0100, 4, 65537, 0)
	}
	putPhotoIFD(tiff[childOffset+2+(childCount-1)*12:], 0x0100, 0xffff, 1, 0)
	photo, err := processPhoto(context.Background(), bytes.NewReader(photoPNGWithEXIF(t, tiff)))
	if err != nil {
		t.Fatal(err)
	}
	assertPhotoCorners(t, decodedPhoto(t, photo, 80, 120), [4]color.RGBA{photoBlue, photoRed, photoYellow, photoGreen})
}

var (
	photoRed    = color.RGBA{R: 255, A: 255}
	photoGreen  = color.RGBA{G: 255, A: 255}
	photoBlue   = color.RGBA{B: 255, A: 255}
	photoYellow = color.RGBA{R: 255, G: 255, A: 255}
)

func quadrantPhoto() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 120, 80))
	for _, quadrant := range []struct {
		bounds image.Rectangle
		color  color.RGBA
	}{
		{image.Rect(0, 0, 60, 40), photoRed}, {image.Rect(60, 0, 120, 40), photoGreen},
		{image.Rect(0, 40, 60, 80), photoBlue}, {image.Rect(60, 40, 120, 80), photoYellow},
	} {
		draw.Draw(img, quadrant.bounds, image.NewUniform(quadrant.color), image.Point{}, draw.Src)
	}
	return img
}

func photoJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func photoPNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func photoPNGWithEXIF(t *testing.T, tiff []byte) []byte {
	t.Helper()
	still := photoPNG(t, quadrantPhoto())
	output := append(bytes.Clone(still[:33]), photoPNGChunk("eXIf", tiff)...)
	return append(output, still[33:]...)
}
func decodedPhoto(t *testing.T, photo models.ProcessedPhoto, width, height int) image.Image {
	t.Helper()
	if photo.Width != width || photo.Height != height {
		t.Fatalf("result dimensions %dx%d, want %dx%d", photo.Width, photo.Height, width, height)
	}
	if len(photo.Bytes) == 0 || len(photo.Bytes) > 4<<20 {
		t.Fatalf("JPEG size %d", len(photo.Bytes))
	}
	output, format, err := image.Decode(bytes.NewReader(photo.Bytes))
	if err != nil || format != "jpeg" {
		t.Fatalf("output format %q: %v", format, err)
	}
	if output.Bounds().Size() != image.Pt(width, height) {
		t.Fatalf("encoded dimensions %v", output.Bounds())
	}
	return output
}
func assertPhotoCorners(t *testing.T, img image.Image, want [4]color.RGBA) {
	t.Helper()
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	for i, point := range []image.Point{{w / 4, h / 4}, {3 * w / 4, h / 4}, {w / 4, 3 * h / 4}} {
		assertPhotoColor(t, img.At(point.X, point.Y), want[i])
	}
	assertPhotoColor(t, img.At(3*w/4, 3*h/4), want[3])
}
func assertPhotoColor(t *testing.T, got color.Color, want color.RGBA) {
	t.Helper()
	actual := color.RGBAModel.Convert(got).(color.RGBA)
	for _, difference := range []int{int(actual.R) - int(want.R), int(actual.G) - int(want.G), int(actual.B) - int(want.B)} {
		if difference < -16 || difference > 16 {
			t.Fatalf("pixel %v, want approximately %v", actual, want)
		}
	}
}
func assertPhotoFault(t *testing.T, err error, code string) {
	t.Helper()
	var fault *models.Fault
	if !errors.As(err, &fault) || fault.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}

// A valid TIFF IFD0 contains Orientation and a GPS IFD with latitude/longitude.
// Embedding it as a JPEG APP1 exercises actual metadata parsing, not an injected
// orientation value or a processor-internal transform.
func withPhotoEXIF(jpegBytes []byte, orientation uint16) []byte {
	exif := append([]byte("Exif\x00\x00"), photoEXIFTIFF(orientation)...)
	output := append([]byte{0xff, 0xd8, 0xff, 0xe1, byte((len(exif) + 2) >> 8), byte(len(exif) + 2)}, exif...)
	return append(output, jpegBytes[2:]...)
}
func photoEXIFTIFF(orientation uint16) []byte {
	tiff := make([]byte, 140)
	copy(tiff, "II")
	binary.LittleEndian.PutUint16(tiff[2:], 42)
	binary.LittleEndian.PutUint32(tiff[4:], 8)
	binary.LittleEndian.PutUint16(tiff[8:], 2)
	putPhotoIFD(tiff[10:], 0x0112, 3, 1, uint32(orientation))
	putPhotoIFD(tiff[22:], 0x8825, 4, 1, 38)
	binary.LittleEndian.PutUint16(tiff[38:], 4)
	putPhotoIFD(tiff[40:], 1, 2, 2, uint32('N'))
	putPhotoIFD(tiff[52:], 2, 5, 3, 92)
	putPhotoIFD(tiff[64:], 3, 2, 2, uint32('E'))
	putPhotoIFD(tiff[76:], 4, 5, 3, 116)
	for i, value := range []uint32{43, 15, 0, 76, 57, 0} {
		binary.LittleEndian.PutUint32(tiff[92+i*8:], value)
		binary.LittleEndian.PutUint32(tiff[96+i*8:], 1)
	}
	return tiff
}
func putPhotoIFD(entry []byte, tag, kind uint16, count, value uint32) {
	binary.LittleEndian.PutUint16(entry, tag)
	binary.LittleEndian.PutUint16(entry[2:], kind)
	binary.LittleEndian.PutUint32(entry[4:], count)
	binary.LittleEndian.PutUint32(entry[8:], value)
}
func photoTags(t *testing.T, input []byte, format imagemeta.ImageFormat) imagemeta.Tags {
	t.Helper()
	var tags imagemeta.Tags
	_, err := imagemeta.Decode(imagemeta.Options{R: bytes.NewReader(input), ImageFormat: format, Sources: imagemeta.EXIF, HandleTag: func(tag imagemeta.TagInfo) error { tags.Add(tag); return nil }})
	if err != nil {
		t.Fatal(err)
	}
	return tags
}

func photoPNGHeader(width, height uint32) []byte {
	header := make([]byte, 13)
	binary.BigEndian.PutUint32(header, width)
	binary.BigEndian.PutUint32(header[4:], height)
	header[8], header[9] = 8, 2
	return append([]byte("\x89PNG\r\n\x1a\n"), photoPNGChunk("IHDR", header)...)
}
func photoPNGChunk(kind string, data []byte) []byte {
	chunk := make([]byte, len(data)+12)
	binary.BigEndian.PutUint32(chunk, uint32(len(data)))
	copy(chunk[4:], kind)
	copy(chunk[8:], data)
	binary.BigEndian.PutUint32(chunk[8+len(data):], crc32.ChecksumIEEE(chunk[4:8+len(data)]))
	return chunk
}
func photoAPNG(still []byte, frames uint32) []byte {
	control := make([]byte, 8)
	binary.BigEndian.PutUint32(control, frames)
	frame := make([]byte, 26)
	binary.BigEndian.PutUint32(frame[4:], 120)
	binary.BigEndian.PutUint32(frame[8:], 80)
	output := append(bytes.Clone(still[:33]), photoPNGChunk("acTL", control)...)
	output = append(output, photoPNGChunk("fcTL", frame)...)
	return append(output, still[33:]...)
}
func photoAnimatedWebP(still []byte, kind string) []byte {
	var payload []byte
	switch kind {
	case "VP8X":
		payload = []byte{0x02, 0, 0, 0, 74, 0, 0, 99, 0, 0}
	case "ANIM":
		payload = make([]byte, 6)
	case "ANMF":
		payload = make([]byte, 16)
	}
	output := append(bytes.Clone(still[:12]), photoWebPChunk(kind, payload)...)
	output = append(output, still[12:]...)
	binary.LittleEndian.PutUint32(output[4:], uint32(len(output)-8))
	return output
}
func photoWebPChunk(kind string, payload []byte) []byte {
	chunk := make([]byte, 8+len(payload)+(len(payload)&1))
	copy(chunk, kind)
	binary.LittleEndian.PutUint32(chunk[4:], uint32(len(payload)))
	copy(chunk[8:], payload)
	return chunk
}

type photoCountingReader struct{ bytes int }

func (r *photoCountingReader) Read(p []byte) (int, error) {
	clear(p)
	r.bytes += len(p)
	return len(p), nil
}

type photoFailingReader struct{}

func (photoFailingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

type photoCancelReader struct {
	cancel context.CancelFunc
	data   []byte
}

type photoBlockedMetadataReader struct {
	*bytes.Reader
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (r *photoBlockedMetadataReader) Read(p []byte) (int, error) {
	r.once.Do(func() { close(r.entered) })
	<-r.release
	return r.Reader.Read(p)
}

func (r photoCancelReader) Read(p []byte) (int, error) { r.cancel(); return copy(p, r.data), io.EOF }

// gopher-doc.1bpp.lossless.webp from golang.org/x/image v0.47.0/testdata:
// https://github.com/golang/image/blob/v0.47.0/testdata/gopher-doc.1bpp.lossless.webp
// Copyright 2009 The Go Authors.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//   - Redistributions of source code must retain the above copyright notice,
//     this list of conditions and the following disclaimer.
//   - Redistributions in binary form must reproduce the above copyright notice,
//     this list of conditions and the following disclaimer in the documentation
//     and/or other materials provided with the distribution.
//   - Neither the name of Google LLC nor the names of its contributors may be
//     used to endorse or promote products derived from this software without
//     specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.
func officialPhotoWebP(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString("UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA==")
	if err != nil {
		t.Fatal(err)
	}
	return data
}
