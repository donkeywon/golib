package lines

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/donkeywon/golib/util/bytespool"
	"github.com/icza/backscanner"
)

const defaultBufSize = 32 * 1024

func ReadLines(r io.Reader, lines int, bufSize int) ([]string, error) {
	if bufSize == 0 {
		bufSize = defaultBufSize
	}
	bs := bytespool.GetN(bufSize)
	defer bs.Free()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(bs.B(), bufSize)

	var finalLines []string

	for lines > 0 && scanner.Scan() {
		finalLines = append(finalLines, scanner.Text())
		lines--
	}
	if scanner.Err() == nil {
		return finalLines, nil
	}
	if errors.Is(scanner.Err(), bufio.ErrTooLong) {
		return finalLines, fmt.Errorf("line too long, exceed %d bytes", bufSize)
	}

	return finalLines, fmt.Errorf("read line fail: %s", scanner.Err().Error())
}

func ReadLinesReverse(r io.ReaderAt, startPos int, lines int, bufSize int) ([]string, int64, error) {
	if bufSize == 0 {
		bufSize = defaultBufSize
	}

	scanner := backscanner.NewOptions(r, startPos, &backscanner.Options{
		MaxBufferSize: bufSize,
	})

	var (
		pos int
		str string
		err error
	)

	var finalLines []string

	for lines > 0 {
		str, pos, err = scanner.Line()
		if err != nil {
			break
		}
		finalLines = append(finalLines, str)
		lines--
	}

	if errors.Is(err, io.EOF) {
		err = nil
	}
	return finalLines, int64(pos), err
}
