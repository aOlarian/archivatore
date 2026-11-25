/* MAP
 * §1 package & imports       L7-L21
 * §2 WriteTarStream (MT)     L23-L148
 * §3 CompressCapsuleTar      L150-L163
 */

// §1 package & imports
package main

import (
	"archive/tar"
	"bufio"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// §2 WriteTarStream (MT)
type prefetchedFile struct {
	entry FileEntry
	data  []byte
	err   error
}

func WriteTarToWriter(w io.Writer, entries []FileEntry, threads int, totalFiles, totalBytes int64, emit *Emitter) error {
	if threads < 1 {
		threads = 1
	}
	bw := bufio.NewWriterSize(w, WriteBufSize)
	tw := tar.NewWriter(bw)

	jobs := make(chan int, threads*4)
	results := make([]chan prefetchedFile, len(entries))
	for i, e := range entries {
		if !e.IsDir && e.Size > 0 && e.Size <= PrefetchMaxFile {
			results[i] = make(chan prefetchedFile, 1)
		}
	}

	var wg sync.WaitGroup
	for t := 0; t < threads; t++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				e := entries[idx]
				data, err := os.ReadFile(WinLongPath(e.AbsPath))
				results[idx] <- prefetchedFile{entry: e, data: data, err: err}
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
		hdr := &tar.Header{
			Format:  tar.FormatPAX,
			Name:    e.RelPath,
			Size:    e.Size,
			Mode:    int64(e.Mode.Perm()),
			ModTime: time.Unix(e.ModTime, 0),
		}
		if e.IsDir {
			hdr.Typeflag = tar.TypeDir
			hdr.Size = 0
			if !strings.HasSuffix(hdr.Name, "/") {
				hdr.Name += "/"
			}
			if hdr.Mode == 0 {
				hdr.Mode = 0755
			}
		} else {
			hdr.Typeflag = tar.TypeReg
			if hdr.Mode == 0 {
				hdr.Mode = 0644
			}
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if e.IsDir {
			continue
		}
		if results[i] != nil {
			pf := <-results[i]
			if pf.err != nil {
				return pf.err
			}
			if _, err := tw.Write(pf.data); err != nil {
				return err
			}
			doneBytes += int64(len(pf.data))
		} else if e.Size > 0 {
			f, err := os.Open(WinLongPath(e.AbsPath))
			if err != nil {
				return err
			}
			n, cErr := io.CopyBuffer(tw, f, copyBuf)
			_ = f.Close()
			if cErr != nil {
				return cErr
			}
			doneBytes += n
		}
		doneFiles++
		if emit != nil {
			emit.EmitProgress(doneFiles, totalFiles, doneBytes, totalBytes, e.RelPath, false)
		}
	}

	if err := tw.Close(); err != nil {
		return err
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	if emit != nil {
		emit.EmitProgress(doneFiles, totalFiles, doneBytes, totalBytes, "", true)
	}
	return nil
}

// §3 CompressCapsuleTar
func CompressCapsuleTar(tmpOut string, entries []FileEntry, threads int, totalFiles, totalBytes int64, emit *Emitter) error {
	f, err := os.OpenFile(WinLongPath(tmpOut), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	err = WriteTarToWriter(f, entries, threads, totalFiles, totalBytes, emit)
	if cErr := f.Close(); err == nil {
		err = cErr
	}
	return err
}
