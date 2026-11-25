/* MAP
 * §1 package & imports    L7-L19
 * §2 ExtractZipMT         L21-L132
 * §3 InspectZipRoots      L134-L157
 */

// §1 package & imports
package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// §2 ExtractZipMT
func ExtractZipMT(archivePath, destDir string, threads int, emit *Emitter) (int64, int64, error) {
	if threads < 1 {
		threads = 1
	}
	zr, err := zip.OpenReader(WinLongPath(archivePath))
	if err != nil {
		return 0, 0, err
	}
	defer zr.Close()

	var totalBytes, totalFiles int64
	var fileItems []*zip.File
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

	for _, f := range zr.File {
		target, cErr := CleanExtractPath(destDir, f.Name)
		if cErr != nil {
			return 0, 0, cErr
		}
		if f.FileInfo().IsDir() || strings.HasSuffix(f.Name, "/") {
			if err := ensureDir(target); err != nil {
				return 0, 0, err
			}
			continue
		}
		if err := ensureDir(filepath.Dir(target)); err != nil {
			return 0, 0, err
		}
		totalFiles++
		totalBytes += int64(f.UncompressedSize64)
		fileItems = append(fileItems, f)
	}

	jobCh := make(chan *zip.File, threads*4)
	var errMu sync.Mutex
	var firstErr error
	var doneFiles, doneBytes int64
	var wg sync.WaitGroup

	for t := 0; t < threads; t++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, 256*1024)
			for f := range jobCh {
				target, _ := CleanExtractPath(destDir, f.Name)
				rc, oErr := f.Open()
				if oErr != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = oErr
					}
					errMu.Unlock()
					continue
				}
				mode := f.Mode() & 0777
				if mode == 0 {
					mode = 0644
				}
				out, cErr := os.OpenFile(WinLongPath(target), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
				if cErr != nil {
					_ = rc.Close()
					errMu.Lock()
					if firstErr == nil {
						firstErr = cErr
					}
					errMu.Unlock()
					continue
				}
				n, wErr := io.CopyBuffer(out, rc, buf)
				_ = rc.Close()
				_ = out.Close()
				if wErr != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = wErr
					}
					errMu.Unlock()
					continue
				}
				_ = os.Chtimes(WinLongPath(target), f.Modified, f.Modified)
				df := atomic.AddInt64(&doneFiles, 1)
				db := atomic.AddInt64(&doneBytes, n)
				if emit != nil {
					emit.EmitProgress(df, totalFiles, db, totalBytes, f.Name, false)
				}
			}
		}()
	}

	for _, f := range fileItems {
		jobCh <- f
	}
	close(jobCh)
	wg.Wait()

	if firstErr != nil {
		return doneFiles, doneBytes, firstErr
	}
	if emit != nil {
		emit.EmitProgress(doneFiles, totalFiles, doneBytes, totalBytes, "", true)
	}
	return doneFiles, doneBytes, nil
}

// §3 InspectZipRoots
func InspectZipRoots(archivePath string) ([]string, error) {
	zr, err := zip.OpenReader(WinLongPath(archivePath))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	seen := make(map[string]bool)
	var roots []string
	for _, f := range zr.File {
		name := strings.TrimLeft(strings.ReplaceAll(f.Name, `\`, "/"), "/")
		if name == "" {
			continue
		}
		parts := strings.SplitN(name, "/", 2)
		if parts[0] != "" && parts[0] != "." && !seen[parts[0]] {
			seen[parts[0]] = true
			roots = append(roots, parts[0])
		}
	}
	return roots, nil
}
