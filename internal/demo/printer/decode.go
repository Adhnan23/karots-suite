// Package printer is the Demo Room's receipt and label printers: TCP listeners
// the shop's settings point at as network printers, so the suite's real print
// path runs unchanged. Each job is decoded back into what the paper would show.
package printer

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"regexp"
	"strconv"

	"karots-suite/internal/escpos"
)

// Paper is one decoded job. Image-mode jobs give a PNG; a Classic (text) job —
// what the suite falls back to when a picture fails — gives its text lines.
type Paper struct {
	PNG    []byte   `json:"-"` // served on its own, not inside the list
	Text   []string `json:"text,omitempty"`
	Kick   bool     `json:"kick"` // ESC p: the cash drawer was told to open
	Copies int      `json:"copies"`
}

// DecodeESCPOS turns a receipt job into the slip: every GS v 0 band stacked
// top to bottom; no raster at all = a Classic job, read as text lines.
func DecodeESCPOS(job []byte) (Paper, error) {
	var p Paper
	var rows [][]byte // packed rows, MSB first
	rowBytes := 0
	for i := 0; i < len(job); i++ {
		switch {
		case job[i] == 0x1B && i+1 < len(job) && job[i+1] == 'p':
			p.Kick = true
		case job[i] == 0x1D && i+7 < len(job) && job[i+1] == 'v' && job[i+2] == '0':
			bpr := int(job[i+4]) + int(job[i+5])*256
			h := int(job[i+6]) + int(job[i+7])*256
			start, end := i+8, i+8+bpr*h
			if end > len(job) {
				return p, errors.New("raster band runs past the end of the job")
			}
			rowBytes = max(rowBytes, bpr)
			for y := 0; y < h; y++ {
				rows = append(rows, job[start+y*bpr:start+(y+1)*bpr])
			}
			i = end - 1
		}
	}
	p.Copies = 1
	if len(rows) == 0 {
		for _, l := range escpos.Preview(job) {
			p.Text = append(p.Text, l.Text)
		}
		return p, nil
	}
	img := image.NewGray(image.Rect(0, 0, rowBytes*8, len(rows)))
	for y, row := range rows {
		for x := 0; x < rowBytes*8; x++ {
			c := color.Gray{Y: 255}
			if x/8 < len(row) && row[x/8]&(0x80>>(x%8)) != 0 {
				c = color.Gray{Y: 0}
			}
			img.SetGray(x, y, c)
		}
	}
	var err error
	p.PNG, err = encode(img)
	return p, err
}

var (
	reSize   = regexp.MustCompile(`SIZE (\d+) mm, (\d+) mm`)
	reBitmap = regexp.MustCompile(`BITMAP (\d+),(\d+),(\d+),(\d+),\d+,`)
	rePrint  = regexp.MustCompile(`PRINT (\d+)`)
	reText   = regexp.MustCompile(`TEXT [^\r\n]*,"((?:[^"\\]|\\.)*)"\r?\n`)
)

// DecodeTSPL turns a label job into the label: the BITMAP (TSPL: 1 = paper,
// 0 = ink) on a SIZE-sized canvas at 8 dots/mm; no BITMAP = a Classic job, read
// as its TEXT values.
func DecodeTSPL(job []byte) (Paper, error) {
	p := Paper{Copies: 1}
	if m := rePrint.FindSubmatch(job); m != nil {
		p.Copies, _ = strconv.Atoi(string(m[1]))
	}
	loc := reBitmap.FindSubmatchIndex(job)
	if loc == nil {
		for _, m := range reText.FindAllSubmatch(job, -1) {
			p.Text = append(p.Text, string(m[1]))
		}
		return p, nil
	}
	num := func(k int) int { n, _ := strconv.Atoi(string(job[loc[2*k]:loc[2*k+1]])); return n }
	x0, y0, bpr, h := num(1), num(2), num(3), num(4)
	data := job[loc[1]:]
	if len(data) < bpr*h {
		return p, errors.New("label bitmap runs past the end of the job")
	}
	w, ht := x0+bpr*8, y0+h
	if m := reSize.FindSubmatch(job); m != nil {
		wm, _ := strconv.Atoi(string(m[1]))
		hm, _ := strconv.Atoi(string(m[2]))
		w, ht = max(w, wm*8), max(ht, hm*8)
	}
	img := image.NewGray(image.Rect(0, 0, w, ht))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	for y := 0; y < h; y++ {
		for x := 0; x < bpr*8; x++ {
			if data[y*bpr+x/8]&(0x80>>(x%8)) == 0 {
				img.SetGray(x0+x, y0+y, color.Gray{Y: 0})
			}
		}
	}
	var err error
	p.PNG, err = encode(img)
	return p, err
}

func encode(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	err := png.Encode(&b, img)
	return b.Bytes(), err
}
