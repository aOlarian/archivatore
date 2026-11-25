/* MAP
 * §1 package & imports       L7-L20
 * §2 ParallelGzipWriter      L22-L137
 * §3 Compress & Extract GZ   L139-L217
 */

// §1 package & imports
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// §2 ParallelGzipWriter
type gzipBlock struct {
	seq  int
	data []byte
	err  error
}

type ParallelGzipWriter struct {
	dst     io.Writer
	level   int
	threads int
	buf     []byte
	seq     int
	jobs    chan gzipBlock
	results map[int]gzipBlock
	resCh   chan gzipBlock
	nextSeq int
	wg      sync.WaitGroup
	writer  sync.WaitGroup
	errMu   sync.Mutex
	err     error
}

func NewParallelGzipWriter(dst io.Writer, level, threads int) *ParallelGzipWriter {
	if threads < 1 {
		threads = 1
	}
	if level < 1 {
		level = 1
	} else if level > 9 {
		level = 9
	}
	pw := &ParallelGzipWriter{
		dst:     dst,
		level:   level,
		threads: threads,
		buf:     make([]byte, 0, GzipBlockSize),
		jobs:    make(chan gzipBlock, threads*2),
		results: make(map[int]gzipBlock),
		resCh:   make(chan gzipBlock, threads*2),
	}
	for t := 0; t < threads; t++ {
		pw.wg.Add(1)
		go func() {
			defer pw.wg.Done()
			for blk := range pw.jobs {
				var out bytes.Buffer
				out.Grow(len(blk.data)/2 + 64)
				gw, err := gzip.NewWriterLevel(&out, pw.level)
				if err == nil {
					_, err = gw.Write(blk.data)
					if cErr := gw.Close(); err == nil {
						err = cErr
					}
				}
				pw.resCh <- gzipBlock{seq: blk.seq, data: out.Bytes(), err: err}
			}
		}()
	}
	pw.writer.Add(1)
	go func() {
		defer pw.writer.Done()
		for res := range pw.resCh {
			pw.results[res.seq] = res
			for {
				ready, ok := pw.results[pw.nextSeq]
				if !ok {
					break
				}
				delete(pw.results, pw.nextSeq)
				pw.nextSeq++
				if ready.err != nil {
					pw.setErr(ready.err)
					continue
				}
				if _, wErr := pw.dst.Write(ready.data); wErr != nil {
					pw.setErr(wErr)
				}
			}
		}
	}()
	return pw
}

func (pw *ParallelGzipWriter) setErr(e error) {
	pw.errMu.Lock()
	if pw.err == nil {
		pw.err = e
	}
	pw.errMu.Unlock()
}

func (pw *ParallelGzipWriter) getErr() error {
	pw.errMu.Lock()
	defer pw.errMu.Unlock()
	return pw.err
}

func (pw *ParallelGzipWriter) flushChunk() {
	if len(pw.buf) == 0 {
		return
	}
	chunk := make([]byte, len(pw.buf))
	copy(chunk, pw.buf)
	pw.buf = pw.buf[:0]
	pw.jobs <- gzipBlock{seq: pw.seq, data: chunk}
	pw.seq++
}

func (pw *ParallelGzipWriter) Write(p []byte) (int, error) {
	if err := pw.getErr(); err != nil {
		return 0, err
	}
	total := len(p)
	for len(p) > 0 {
		avail := GzipBlockSize - len(pw.buf)
		if len(p) < avail {
			pw.buf = append(pw.buf, p...)
			break
		}
		pw.buf = append(pw.buf, p[:avail]...)
		p = p[avail:]
		pw.flushChunk()
	}
	return total, pw.getErr()
}

func (pw *ParallelGzipWriter) Close() error {
	if len(pw.buf) > 0 || pw.seq == 0 {
		chunk := make([]byte, len(pw.buf))
		copy(chunk, pw.buf)
		pw.buf = pw.buf[:0]
		pw.jobs <- gzipBlock{seq: pw.seq, data: chunk}
		pw.seq++
	}
	close(pw.jobs)
	pw.wg.Wait()
	close(pw.resCh)
	pw.writer.Wait()
	return pw.getErr()
}

// §3 Compress & Extract GZ
func CompressTarGzipMT(tmpOut string, entries []FileEntry, level, threads int, totalFiles, totalBytes int64, emit *Emitter) error {
	f, err := os.OpenFile(WinLongPath(tmpOut), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(f, WriteBufSize)
	pgw := NewParallelGzipWriter(bw, level, threads)
	err = WriteTarToWriter(pgw, entries, threads, totalFiles, totalBytes, emit)
	if cErr := pgw.Close(); err == nil {
		err = cErr
	}
	if fErr := bw.Flush(); err == nil {
		err = fErr
	}
	if cErr := f.Close(); err == nil {
		err = cErr
	}
	return err
}

func CompressSingleGzipMT(srcPath, tmpOut string, level, threads int, emit *Emitter) (int64, error) {
	inF, err := os.Open(WinLongPath(srcPath))
	if err != nil {
		return 0, err
	}
	defer inF.Close()
	info, err := inF.Stat()
	if err != nil {
		return 0, err
	}
	outF, err := os.OpenFile(WinLongPath(tmpOut), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, err
	}
	bw := bufio.NewWriterSize(outF, WriteBufSize)
	pgw := NewParallelGzipWriter(bw, level, threads)
	buf := make([]byte, StreamCopyBufSize)
	var done int64
	total := info.Size()
	for {
		n, rErr := inF.Read(buf)
		if n > 0 {
			if _, wErr := pgw.Write(buf[:n]); wErr != nil {
				_ = outF.Close()
				return done, wErr
			}
			done += int64(n)
			if emit != nil {
				emit.EmitProgress(0, 1, done, total, filepath.Base(srcPath), false)
			}
		}
		if rErr == io.EOF {
			break
		}
		if rErr != nil {
			_ = outF.Close()
			return done, rErr
		}
	}
	if err := pgw.Close(); err != nil {
		_ = outF.Close()
		return done, err
	}
	if err := bw.Flush(); err != nil {
		_ = outF.Close()
		return done, err
	}
	if emit != nil {
		emit.EmitProgress(1, 1, done, total, "", true)
	}
	return done, outF.Close()
}

func ExtractTarGzip(archivePath, destDir string, threads int, emit *Emitter) (int64, int64, error) {
	f, err := os.Open(WinLongPath(archivePath))
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	info, _ := f.Stat()
	var totalBytes int64
	if info != nil {
		totalBytes = info.Size()
	}
	gr, err := gzip.NewReader(bufio.NewReaderSize(f, WriteBufSize))
	if err != nil {
		return 0, 0, err
	}
	gr.Multistream(true)
	defer gr.Close()
	return ExtractTarStream(gr, destDir, threads, totalBytes, emit)
}

func ExtractSingleGzip(archivePath, destDir, conflict string, emit *Emitter) (int64, int64, error) {
	inF, err := os.Open(WinLongPath(archivePath))
	if err != nil {
		return 0, 0, err
	}
	defer inF.Close()
	gr, err := gzip.NewReader(bufio.NewReaderSize(inF, WriteBufSize))
	if err != nil {
		return 0, 0, err
	}
	gr.Multistream(true)
	defer gr.Close()

	base := filepath.Base(archivePath)
	outName := strings.TrimSuffix(base, filepath.Ext(base))
	if gr.Name != "" {
		outName = filepath.Base(gr.Name)
	}
	ext := filepath.Ext(outName)
	stem := strings.TrimSuffix(outName, ext)
	finalOut, skipped, err := ResolveOutputPath(stem, destDir, ext, conflict)
	if err != nil || skipped {
		return 0, 0, err
	}
	tmpOut := finalOut + ".archivatore-tmp"
	outF, err := os.OpenFile(WinLongPath(tmpOut), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, 0, err
	}
	n, cErr := io.CopyBuffer(outF, gr, make([]byte, StreamCopyBufSize))
	_ = outF.Close()
	if cErr != nil {
		_ = os.Remove(WinLongPath(tmpOut))
		return 0, 0, cErr
	}
	if err := os.Rename(WinLongPath(tmpOut), WinLongPath(finalOut)); err != nil {
		_ = os.Remove(WinLongPath(tmpOut))
		return 0, 0, err
	}
	if emit != nil {
		emit.EmitProgress(1, 1, n, n, outName, true)
	}
	return 1, n, nil
}
