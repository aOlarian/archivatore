/* MAP
 * §1 package & imports       L7-L21
 * §2 ExtractTarStream (MT)   L23-L143
 * §3 InspectTarStreamRoots   L145-L173
 */

// §1 package & imports
package main

import (
	"archive/tar"
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// §2 ExtractTarStream (MT)
type tarWriteJob struct {
	target  string
	data    []byte
	mode    os.FileMode
	modTime time.Time
}

func ExtractTarStream(r io.Reader, destDir string, threads int, totalBytes int64, emit *Emitter) (int64, int64, error) {
	if threads < 1 {
		threads = 1
	}
	tr := tar.NewReader(bufio.NewReaderSize(r, WriteBufSize))
	madeDirs := make(map[string]bool)
	ensureDir := func(d string) error {
		if madeDirs[d] {
			return nil
		}
		if err := os.MkdirAll(WinLongPath(d), 0755); err != nil {
			return err
		}
		madeDirs[d] = true
		return nil
	}
	if err := ensureDir(destDir); err != nil {
		return 0, 0, err
	}

	jobCh := make(chan tarWriteJob, threads*4)
	var errMu sync.Mutex
	var firstErr error
	var doneFiles, doneBytes int64
	var wg sync.WaitGroup

	for t := 0; t < threads; t++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobCh {
				if err := os.WriteFile(WinLongPath(j.target), j.data, j.mode); err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
					continue
				}
				_ = os.Chtimes(WinLongPath(j.target), j.modTime, j.modTime)
				df := atomic.AddInt64(&doneFiles, 1)
				db := atomic.AddInt64(&doneBytes, int64(len(j.data)))
				if emit != nil {
					emit.EmitProgress(df, 0, db, totalBytes, filepath.Base(j.target), false)
				}
			}
		}()
	}

	copyBuf := make([]byte, StreamCopyBufSize)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			close(jobCh)
			wg.Wait()
			return doneFiles, doneBytes, err
		}
		target, cErr := CleanExtractPath(destDir, hdr.Name)
		if cErr != nil {
			close(jobCh)
			wg.Wait()
			return doneFiles, doneBytes, cErr
		}
		if hdr.Typeflag == tar.TypeDir {
			if err := ensureDir(target); err != nil {
				close(jobCh)
				wg.Wait()
				return doneFiles, doneBytes, err
			}
			continue
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		if err := ensureDir(filepath.Dir(target)); err != nil {
			close(jobCh)
			wg.Wait()
			return doneFiles, doneBytes, err
		}
		mode := os.FileMode(hdr.Mode) & 0777
		if mode == 0 {
			mode = 0644
		}
		if hdr.Size <= ExtractSmallMax {
			data, rErr := io.ReadAll(tr)
			if rErr != nil {
				close(jobCh)
				wg.Wait()
				return doneFiles, doneBytes, rErr
			}
			jobCh <- tarWriteJob{target: target, data: data, mode: mode, modTime: hdr.ModTime}
			continue
		}
		outF, oErr := os.OpenFile(WinLongPath(target), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if oErr != nil {
			close(jobCh)
			wg.Wait()
			return doneFiles, doneBytes, oErr
		}
		n, wErr := io.CopyBuffer(outF, tr, copyBuf)
		_ = outF.Close()
		if wErr != nil {
			close(jobCh)
			wg.Wait()
			return doneFiles, doneBytes, wErr
		}
		_ = os.Chtimes(WinLongPath(target), hdr.ModTime, hdr.ModTime)
		df := atomic.AddInt64(&doneFiles, 1)
		db := atomic.AddInt64(&doneBytes, n)
		if emit != nil {
			emit.EmitProgress(df, 0, db, totalBytes, hdr.Name, false)
		}
	}

	close(jobCh)
	wg.Wait()
	if firstErr != nil {
		return doneFiles, doneBytes, firstErr
	}
	if emit != nil {
		emit.EmitProgress(doneFiles, doneFiles, doneBytes, totalBytes, "", true)
	}
	return doneFiles, doneBytes, nil
}

// §3 InspectTarStreamRoots
func InspectTarStreamRoots(r io.Reader) ([]string, error) {
	tr := tar.NewReader(bufio.NewReaderSize(r, 256*1024))
	seen := make(map[string]bool)
	var roots []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimLeft(strings.ReplaceAll(hdr.Name, `\`, "/"), "/")
		if name == "" {
			continue
		}
		parts := strings.SplitN(name, "/", 2)
		top := parts[0]
		if top != "" && top != "." && !seen[top] {
			seen[top] = true
			roots = append(roots, top)
		}
	}
	return roots, nil
}
