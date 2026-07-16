package terminal

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

func readPasswordLine(file *os.File) ([]byte, error) {
	line, err := bufio.NewReader(file).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if err != nil && len(line) == 0 {
		return nil, err
	}
	return []byte(strings.TrimRight(line, "\r\n")), nil
}
