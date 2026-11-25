/* MAP
 * §1 package & imports         L7-L19
 * §2 Archive format detection  L21-L72
 * §3 Smart extraction logic    L74-L203
 */

// §1 package & imports
package main

import (
	"bufio"
	"compress/gzip"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// §2 Archive format detection
func DetectArchiveFormat(path string) (format string, stem string) {
	name := filepath.Base(path)
	lower := strings.ToLower(name)
	multiExts := []struct {
		suffix string
		format string
	}{
		{".tar.gz", "tar-gz"},
		{".tar.zst", "tar-zst"},
		{".tar.xz", "tar-xz"},
		{".tar.bz2", "tar-bz2"},
		{".tgz", "tar-gz"},
		{".tzst", "tar-zst"},
		{".txz", "tar-xz"},
		{".tbz2", "tar-bz2"},
		{".tar", "tar"},
		{".zip", "zip"},
		{".7z", "7z"},
		{".rar", "rar"},
		{".gz", "gz"},
		{".zst", "zst"},
	}
	for _, m := range multiExts {
		if strings.HasSuffix(lower, m.suffix) {
			return m.format, name[:len(name)-len(m.suffix)]
		}
	}
	return "", name
}

func inspectRoots(tools ToolPaths, format, archivePath string) ([]string, error) {
	switch format {
	case "zip":
		return InspectZipRoots(archivePath)
	case "tar":
		f, err := os.Open(WinLongPath(archivePath))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return InspectTarStreamRoots(f)
	case "tar-gz":
		f, err := os.Open(WinLongPath(archivePath))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		gr, err := gzip.NewReader(bufio.NewReaderSize(f, 256*1024))
		if err != nil {
			return nil, err
		}
		gr.Multistream(true)
		defer gr.Close()
		return InspectTarStreamRoots(gr)
	case "tar-zst":
		if tools.Zstd != "" {
			f, err := os.Open(WinLongPath(archivePath))
			if err != nil {
				return nil, err
			}
			defer f.Close()
			cmd := exec.Command(tools.Zstd, "-d", "-c")
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			cmd.Stdin = f
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				return nil, err
			}
			if err := cmd.Start(); err != nil {
				return nil, err
			}
			roots, rErr := InspectTarStreamRoots(stdout)
			wErr := cmd.Wait()
			if rErr != nil {
				return nil, rErr
			}
			if wErr != nil {
				return nil, wErr
			}
			return roots, nil
		}
		return ListExternalRoots(tools, format, archivePath)
	default:
		return ListExternalRoots(tools, format, archivePath)
	}
}

// §3 Smart extraction logic
func resolveFreeSubdir(baseDir, name, conflict string) (string, bool, error) {
	target := filepath.Join(baseDir, name)
	_, err := os.Stat(WinLongPath(target))
	if os.IsNotExist(err) {
		return target, false, nil
	}
	switch conflict {
	case "overwrite":
		return target, false, nil
	case "skip":
		return target, true, nil
	default:
		for i := 1; i < 100000; i++ {
			cand := filepath.Join(baseDir, name+"-"+itoa(i))
			if _, e := os.Stat(WinLongPath(cand)); os.IsNotExist(e) {
				return cand, false, nil
			}
		}
		return "", false, errors.New("no free destination folder name")
	}
}

func runExtractFormat(tools ToolPaths, format, archivePath, destDir, conflict string, threads int, emit *Emitter) (int64, int64, error) {
	switch format {
	case "tar":
		f, err := os.Open(WinLongPath(archivePath))
		if err != nil {
			return 0, 0, err
		}
		defer f.Close()
		info, _ := f.Stat()
		var sz int64
		if info != nil {
			sz = info.Size()
		}
		return ExtractTarStream(f, destDir, threads, sz, emit)
	case "zip":
		return ExtractZipMT(archivePath, destDir, threads, emit)
	case "tar-gz":
		return ExtractTarGzip(archivePath, destDir, threads, emit)
	case "gz":
		return ExtractSingleGzip(archivePath, destDir, conflict, emit)
	case "tar-zst":
		return ExtractTarZst(tools, archivePath, destDir, threads, emit)
	case "zst":
		return ExtractSingleZst(tools, archivePath, destDir, conflict, threads, emit)
	case "7z", "rar", "tar-xz", "tar-bz2":
		return ExtractWithExternalTool(tools, format, archivePath, destDir, threads, emit)
	default:
		return 0, 0, errors.New("unsupported archive format: " + format)
	}
}

func ExecuteExtract(tools ToolPaths, archivePath, outDir, mode, conflict string, threads int, emit *Emitter) error {
	if outDir == "" {
		outDir = filepath.Dir(archivePath)
	}
	format, stem := DetectArchiveFormat(archivePath)
	if format == "" {
		return errors.New("unrecognized archive format: " + filepath.Base(archivePath))
	}
	inInfo, err := os.Stat(WinLongPath(archivePath))
	if err != nil {
		return err
	}
	inBytes := inInfo.Size()

	// Single-file compressed streams extract directly into outDir
	if format == "gz" || format == "zst" {
		files, bytesOut, eErr := runExtractFormat(tools, format, archivePath, outDir, conflict, threads, emit)
		if eErr != nil {
			return eErr
		}
		emit.EmitDone(outDir, inBytes, bytesOut, files, files == 0)
		return nil
	}

	destDir := outDir
	useTempRename := false
	singleRootName := ""

	switch mode {
	case "direct":
		destDir = outDir
	case "folder":
		var skipped bool
		destDir, skipped, err = resolveFreeSubdir(outDir, stem, conflict)
		if err != nil {
			return err
		}
		if skipped {
			emit.EmitDone(destDir, inBytes, 0, 0, true)
			return nil
		}
	default: // "smart"
		roots, rErr := inspectRoots(tools, format, archivePath)
		if rErr == nil && len(roots) == 1 {
			singleRootName = roots[0]
			if strings.ContainsAny(singleRootName, "?\ufffd") && stem != "" {
				singleRootName = stem
			}
			candidate := filepath.Join(outDir, singleRootName)
			if _, sErr := os.Stat(WinLongPath(candidate)); sErr == nil {
				if conflict == "skip" {
					emit.EmitDone(candidate, inBytes, 0, 0, true)
					return nil
				}
				if conflict == "rename" {
					useTempRename = true
				}
			}
			destDir = outDir
		} else {
			var skipped bool
			destDir, skipped, err = resolveFreeSubdir(outDir, stem, conflict)
			if err != nil {
				return err
			}
			if skipped {
				emit.EmitDone(destDir, inBytes, 0, 0, true)
				return nil
			}
		}
	}

	if useTempRename {
		tmpStage, mErr := os.MkdirTemp(WinLongPath(outDir), ".archivatore-ext-*")
		if mErr != nil {
			return mErr
		}
		defer os.RemoveAll(tmpStage)
		files, bytesOut, eErr := runExtractFormat(tools, format, archivePath, tmpStage, "overwrite", threads, emit)
		if eErr != nil {
			return eErr
		}
		freeTarget, _, fErr := resolveFreeSubdir(outDir, singleRootName, "rename")
		if fErr != nil {
			return fErr
		}
		extractedRoot := filepath.Join(tmpStage, singleRootName)
		if rErr := os.Rename(WinLongPath(extractedRoot), WinLongPath(freeTarget)); rErr != nil {
			return rErr
		}
		emit.EmitDone(freeTarget, inBytes, bytesOut, files, false)
		return nil
	}

	files, bytesOut, eErr := runExtractFormat(tools, format, archivePath, destDir, conflict, threads, emit)
	if eErr != nil {
		return eErr
	}
	finalOut := destDir
	if singleRootName != "" && mode == "smart" {
		finalOut = filepath.Join(outDir, singleRootName)
	}
	emit.EmitDone(finalOut, inBytes, bytesOut, files, false)
	return nil
}
