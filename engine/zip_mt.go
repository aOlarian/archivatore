/* MAP
 * §1 package & imports    L7-L21
 * §2 CompressZipMT        L23-L189
 */

// §1 package & imports
package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/flate"
	"hash/crc32"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// §2 CompressZipMT
type zipPackedItem struct {
	entry    FileEntry
	rawData  []byte
	crc      uint32
	uncompSz uint64
	err      error
}

func compressFlateChunk(raw []byte, level int) ([]byte, uint32, error) {
	crc := crc32.ChecksumIEEE(raw)
	var buf bytes.Buffer
	buf.Grow(len(raw)/2 + 64)
	fw, err := flate.NewWriter(&buf, level)
	if err != nil {
		return nil, 0, err
	}
	if _, err := fw.Write(raw); err != nil {
		return nil, 0, err
	}
	if err := fw.Close(); err != nil {
		return nil, 0, err
	}
	return buf.Bytes(), crc, nil
}

func CompressZipMT(tmpOut string, entries []FileEntry, level, threads int, totalFiles, totalBytes int64, emit *Emitter) error {
	if threads < 1 {
		threads = 1
	}
	if level < 0 {
		level = 0
	} else if level > 9 {
		level = 9
	}

	outF, err := os.OpenFile(WinLongPath(tmpOut), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(outF, WriteBufSize)
	zw := zip.NewWriter(bw)
	if level > 0 {
		zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
			return flate.NewWriter(out, level)
		})
	}

	jobs := make(chan int, threads*4)
	results := make([]chan zipPackedItem, len(entries))
	for i, e := range entries {
		if !e.IsDir && e.Size > 0 && e.Size <= ZipParallelMaxFile {
			results[i] = make(chan zipPackedItem, 1)
		}
	}

	var wg sync.WaitGroup
	for t := 0; t < threads; t++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				e := entries[idx]
				raw, rErr := os.ReadFile(WinLongPath(e.AbsPath))
				if rErr != nil {
					results[idx] <- zipPackedItem{entry: e, err: rErr}
					continue
				}
				if level == 0 {
					results[idx] <- zipPackedItem{
						entry: e, rawData: raw, crc: crc32.ChecksumIEEE(raw), uncompSz: uint64(len(raw)),
					}
					continue
				}
				comp, crc, cErr := compressFlateChunk(raw, level)
				results[idx] <- zipPackedItem{
					entry: e, rawData: comp, crc: crc, uncompSz: uint64(len(raw)), err: cErr,
				}
			}
		}()
	}
	go func() {
		for i := range entries {
			if results[i] != nil {
				jobs <- i
			}
		}
		close(jobs)
		wg.Wait()
	}()

	copyBuf := make([]byte, StreamCopyBufSize)
	var doneFiles, doneBytes int64

	for i, e := range entries {
		hdr := &zip.FileHeader{
			Name:     e.RelPath,
			Modified: time.Unix(e.ModTime, 0),
		}
		if e.IsDir {
			if !strings.HasSuffix(hdr.Name, "/") {
				hdr.Name += "/"
			}
			hdr.Method = zip.Store
			hdr.SetMode(os.ModeDir | 0755)
			if _, err := zw.CreateHeader(hdr); err != nil {
				_ = outF.Close()
				return err
			}
			continue
		}
		hdr.SetMode(e.Mode)
		if level == 0 {
			hdr.Method = zip.Store
		} else {
			hdr.Method = zip.Deflate
		}

		if results[i] != nil {
			item := <-results[i]
			if item.err != nil {
				_ = outF.Close()
				return item.err
			}
			hdr.CRC32 = item.crc
			hdr.UncompressedSize64 = item.uncompSz
			hdr.CompressedSize64 = uint64(len(item.rawData))
			w, wErr := zw.CreateRaw(hdr)
			if wErr != nil {
				_ = outF.Close()
				return wErr
			}
			if _, wErr = w.Write(item.rawData); wErr != nil {
				_ = outF.Close()
				return wErr
			}
			doneBytes += int64(item.uncompSz)
		} else {
			w, wErr := zw.CreateHeader(hdr)
			if wErr != nil {
				_ = outF.Close()
				return wErr
			}
			if e.Size > 0 {
				srcF, oErr := os.Open(WinLongPath(e.AbsPath))
				if oErr != nil {
					_ = outF.Close()
					return oErr
				}
				n, cErr := io.CopyBuffer(w, srcF, copyBuf)
				_ = srcF.Close()
				if cErr != nil {
					_ = outF.Close()
					return cErr
				}
				doneBytes += n
			}
		}
		doneFiles++
		if emit != nil {
			emit.EmitProgress(doneFiles, totalFiles, doneBytes, totalBytes, e.RelPath, false)
		}
	}

	if err := zw.Close(); err != nil {
		_ = outF.Close()
		return err
	}
	if err := bw.Flush(); err != nil {
		_ = outF.Close()
		return err
	}
	if emit != nil {
		emit.EmitProgress(doneFiles, totalFiles, doneBytes, totalBytes, "", true)
	}
	return outF.Close()
}
