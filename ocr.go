package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"math/rand"

	"github.com/gen2brain/go-fitz"
	"github.com/otiai10/gosseract/v2"
)

func (p *Pdf) UseOCR() (bool, error) {
	fitz_doc, err := fitz.New(p.Filename)
	if err != nil {
		return false, err
	}
	defer fitz_doc.Close()

	n_pages := fitz_doc.NumPage()
	n_samples := int(SAMPLE_SIZE * float32(n_pages))

	if n_samples > SAMPLE_MAX {
		n_samples = SAMPLE_MAX
	}

	randomPages := make([]int, n_samples)
	for i := range randomPages {
		randomPages[i] = rand.Intn(n_pages)
	}

	type result struct {
		ocrBetter bool
		err       error
	}

	jobs := make(chan int, n_samples)
	results := make(chan result, n_samples)

	for w := 0; w < n_workers; w++ {
		go func() {
			for sample := range jobs {
				img, err := fitz_doc.Image(sample)
				if err != nil {
					results <- result{ocrBetter: false, err: err}
					continue
				}

				ocr_text, err := image_to_text(img)
				if err != nil {
					results <- result{ocrBetter: false, err: nil}
					continue
				}

				doc_text, err := fitz_doc.Text(sample)
				if err != nil {
					results <- result{ocrBetter: true, err: nil}
					continue
				}

				ocr_quality, valid := TextQuality(ocr_text)
				if !valid {
					results <- result{ocrBetter: false, err: nil}
					continue
				}

				doc_quality, valid := TextQuality(doc_text)
				if !valid {
					results <- result{ocrBetter: true, err: nil}
					continue
				}

				if len(ocr_text) != len(doc_text) {
					if len(ocr_text) > len(doc_text) {
						ocr_quality *= 1.2
					} else {
						doc_quality *= 1.2
					}
				}

				results <- result{ocrBetter: ocr_quality > doc_quality, err: nil}
			}
		}()
	}

	go func() {
		for _, page := range randomPages {
			jobs <- page
		}
		close(jobs)
	}()

	ocr_better := 0
	doc_better := 0

	for i := 0; i < n_samples; i++ {
		res := <-results
		if res.err != nil {
			return false, res.err
		}
		if res.ocrBetter {
			ocr_better++
		} else {
			doc_better++
		}
	}

	fmt.Println("Doc Quality:", doc_better, "\tOCR Quality:", ocr_better)
	return ocr_better > doc_better+1, nil
}

func Img2Text(ocr_client *gosseract.Client, img *image.RGBA) (string, error) {
	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, img, &jpeg.Options{Quality: QUALITY})
	if err != nil {
		return "", err
	}

	ocr_client.SetImageFromBytes(buf.Bytes())

	text, err := ocr_client.Text()
	if err != nil {
		return "", err
	}

	return text, nil
}

// This function should only be called from UseOCR, it is not fully optimized
func image_to_text(img *image.RGBA) (string, error) {
	ocr_client := gosseract.NewClient()
	defer ocr_client.Close()

	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, img, &jpeg.Options{Quality: QUALITY})
	if err != nil {
		return "", err
	}

	ocr_client.SetImageFromBytes(buf.Bytes())

	text, err := ocr_client.Text()
	if err != nil {
		return "", err
	}

	return text, nil
}
