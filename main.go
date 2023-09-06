package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var defaultBufSize = os.Getpagesize()

func writeWithIndent(writer io.Writer, input string, indent int) error {
	builder := strings.Builder{}
	builder.Grow(len(input) + indent + 1)
	for i := 0; i < indent; i += 1 {
		builder.WriteByte(' ')
	}
	builder.WriteString(input + "\n")
	_, err := writer.Write([]byte(builder.String()))
	return err

}

func createBuf(length int) []byte {
	minBufSize := int(unsafe.Sizeof(syscall.Dirent{}))
	if length < minBufSize {
		return make([]byte, minBufSize)
	}
	return make([]byte, length)

}

func BufWriteTree(output io.Writer, path string, buf []byte) error {
	return bufWriteTree(output, path, 0, buf)
}

func bufWriteTree(output io.Writer, path string, indent int, buf []byte) error {
	root, err := os.Open(path)
	if err != nil {
		return err
	}
	defer root.Close()
	fd := int(root.Fd())

	os.Chdir(path)
	currentDir, _ := os.Getwd()

	var trimmedBuf []byte
	var entry syscall.Dirent

	for {
		if len(trimmedBuf) == 0 {
			n, err := syscall.ReadDirent(fd, buf)
			if err != nil {
				return err
			}

			if n <= 0 {
				return nil
			}
			trimmedBuf = buf[:n]
		}
		copy((*[unsafe.Sizeof(entry)]byte)(unsafe.Pointer(&entry))[:], trimmedBuf)
		trimmedBuf = trimmedBuf[entry.Reclen:]

		// skip removed file
		if entry.Ino == 0 {
			continue
		}

		// find null-terminator
		var namelen int
		for i, v := range entry.Name {
			if v == 0 {
				namelen = i
				break
			}
		}
		filename := string((*(*[256]byte)(unsafe.Pointer(&entry.Name)))[:namelen])

		if filename == ".." || filename == "." {
			continue
		}

		if entry.Type == syscall.DT_DIR {
			if err := writeWithIndent(output, filename+"/", indent); err != nil {
				return err
			}
			if err := bufWriteTree(output, filename, indent+1, createBuf(defaultBufSize)); err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
			}
			os.Chdir(currentDir)
			continue
		}
		if err := writeWithIndent(output, filename, indent); err != nil {
			return err
		}

	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "USAGE:\n%s PATH OUTPUT\n", os.Args[0])
		return
	}
	rootPath, outputPath := os.Args[1], os.Args[2]
	outputFile, err := os.Create(outputPath)
	if err != nil {
		panic(err)
	}
	defer outputFile.Close()

	writer := bufio.NewWriterSize(outputFile, defaultBufSize)
	defer writer.Flush()

	if err := BufWriteTree(writer, rootPath, createBuf(defaultBufSize)); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}

}
