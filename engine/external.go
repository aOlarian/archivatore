/* MAP
 * §1 package & imports         L7-L23
 * §2 External runner & helpers L25-L120
 * §3 Zstd compress & extract   L122-L265
 */

// §1 package & imports
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// §2 External runner & helpers
func newHiddenCmd(exe string, args ...string) *exec.Cmd {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func runExternalWithMonitor(cmd *exec.Cmd, watchPath string, totalFiles, totalBytes int64, emit *Emitter) error {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	doneCh := make(chan error, 1)
	go func() { doneCh <- cmd.Wait() }()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-doneCh:
			if err != nil {
				msg := strings.TrimSpace(stderr.String())
				if msg == "" {
					msg = err.Error()
				}
				return fmt.Errorf("%s failed: %s", filepath.Base(cmd.Path), msg)
			}
			return nil
		case <-ticker.C:
			if watchPath != "" && emit != nil {
				if info, sErr := os.Stat(WinLongPath(watchPath)); sErr == nil {
					emit.EmitProgress(0, totalFiles, info.Size(), totalBytes, filepath.Base(watchPath), true)
				}
			}
		}
	}
}

func mapZstdLevel(level int) int {
	table := []int{1, 1, 2, 3, 6, 9, 12, 15, 17, 19}
	if level < 0 {
		return 1
	}
	if level > 9 {
		return 19
	}
	return table[level]
}

func mapRarLevel(level int) int {
	if level <= 0 {
		return 0
	}
	if level <= 2 {
		return 1
	}
	if level <= 4 {
		return 2
	}
	if level <= 6 {
		return 3
	}
	if level <= 8 {
		return 4
	}
	return 5
}

func linkOrTempASCII(srcPath, ext string) (tmpPath string, cleanup func(), err error) {
	dir := filepath.Dir(srcPath)
	tmpFile, err := os.CreateTemp(WinLongPath(dir), "arctmp-*"+ext)
	if err != nil {
		tmpFile, err = os.CreateTemp("", "arctmp-*"+ext)
		if err != nil {
			return "", func() {}, err
		}
	}
	tmpPath = tmpFile.Name()
	_ = tmpFile.Close()
	_ = os.Remove(WinLongPath(tmpPath))
	if lErr := os.Link(WinLongPath(srcPath), WinLongPath(tmpPath)); lErr == nil {
		return tmpPath, func() { _ = os.Remove(WinLongPath(tmpPath)) }, nil
	}
	inF, oErr := os.Open(WinLongPath(srcPath))
	if oErr != nil {
		return "", func() {}, oErr
	}
	defer inF.Close()
	outF, cErr := os.OpenFile(WinLongPath(tmpPath), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if cErr != nil {
		return "", func() {}, cErr
	}
	_, cpErr := io.CopyBuffer(outF, inF, make([]byte, StreamCopyBufSize))
	_ = outF.Close()
	if cpErr != nil {
		_ = os.Remove(WinLongPath(tmpPath))
		return "", func() {}, cpErr
	}
	return tmpPath, func() { _ = os.Remove(WinLongPath(tmpPath)) }, nil
}

// §3 Zstd compress & extract
func CompressTarZst(tools ToolPaths, srcPath, tmpOut string, entries []FileEntry, level, threads int, totalFiles, totalBytes int64, emit *Emitter) error {
	zLvl := mapZstdLevel(level)
	if tools.Zstd != "" {
		outF, oErr := os.OpenFile(WinLongPath(tmpOut), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if oErr != nil {
			return oErr
		}
		cmd := newHiddenCmd(tools.Zstd, fmt.Sprintf("-T%d", threads), fmt.Sprintf("-%d", zLvl), "--no-progress", "-c")
		stdin, err := cmd.StdinPipe()
		if err != nil {
			_ = outF.Close()
			return err
		}
		cmd.Stdout = outF
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Start(); err != nil {
			_ = outF.Close()
			return err
		}
		tarErr := WriteTarToWriter(stdin, entries, threads, totalFiles, totalBytes, emit)
		_ = stdin.Close()
		waitErr := cmd.Wait()
		closeErr := outF.Close()
		if waitErr != nil {
			return fmt.Errorf("zstd failed: %s", strings.TrimSpace(stderr.String()))
		}
		if tarErr != nil {
			return tarErr
		}
		return closeErr
	}
	if tools.Tar == "" {
		return errors.New("neither zstd.exe nor tar.exe found")
	}
	opts := fmt.Sprintf("zstd:compression-level=%d,zstd:threads=%d", zLvl, threads)
	return runTarRepackage(tools, tmpOut, []string{"--zstd", "--options=" + opts, "-cf", "-", "@-"}, entries, totalFiles, totalBytes, emit)
}

func CompressSingleZst(tools ToolPaths, srcPath, tmpOut string, level, threads int, emit *Emitter) error {
	if tools.Zstd == "" {
		return errors.New("zstd.exe not found for single-file .zst compression")
	}
	inF, err := os.Open(WinLongPath(srcPath))
	if err != nil {
		return err
	}
	defer inF.Close()
	outF, err := os.OpenFile(WinLongPath(tmpOut), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer outF.Close()
	zLvl := mapZstdLevel(level)
	cmd := newHiddenCmd(tools.Zstd, fmt.Sprintf("-T%d", threads), fmt.Sprintf("-%d", zLvl), "--no-progress", "-c")
	cmd.Stdin = inF
	cmd.Stdout = outF
	info, _ := inF.Stat()
	var sz int64
	if info != nil {
		sz = info.Size()
	}
	return runExternalWithMonitor(cmd, tmpOut, 1, sz, emit)
}

func ExtractTarZst(tools ToolPaths, archivePath, destDir string, threads int, emit *Emitter) (int64, int64, error) {
	info, _ := os.Stat(WinLongPath(archivePath))
	var totalBytes int64
	if info != nil {
		totalBytes = info.Size()
	}
	if tools.Zstd != "" {
		inF, oErr := os.Open(WinLongPath(archivePath))
		if oErr != nil {
			return 0, 0, oErr
		}
		defer inF.Close()
		cmd := newHiddenCmd(tools.Zstd, "-d", fmt.Sprintf("-T%d", threads), "-c")
		cmd.Stdin = inF
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return 0, 0, err
		}
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Start(); err != nil {
			return 0, 0, err
		}
		files, bytesOut, tarErr := ExtractTarStream(stdout, destDir, threads, totalBytes, emit)
		waitErr := cmd.Wait()
		if waitErr != nil {
			return files, bytesOut, fmt.Errorf("zstd extract failed: %s", strings.TrimSpace(stderr.String()))
		}
		if tarErr != nil {
			return files, bytesOut, tarErr
		}
		return files, bytesOut, nil
	}
	if tools.Tar == "" {
		return 0, 0, errors.New("neither zstd.exe nor tar.exe found")
	}
	if err := os.MkdirAll(WinLongPath(destDir), 0755); err != nil {
		return 0, 0, err
	}
	cmd := newHiddenCmd(tools.Tar, "-xf", archivePath, "-C", destDir)
	if err := runExternalWithMonitor(cmd, "", 0, totalBytes, emit); err != nil {
		return 0, 0, err
	}
	return 0, totalBytes, nil
}

func ExtractSingleZst(tools ToolPaths, archivePath, destDir, conflict string, threads int, emit *Emitter) (int64, int64, error) {
	if tools.Zstd == "" {
		return 0, 0, errors.New("zstd.exe not found")
	}
	base := filepath.Base(archivePath)
	outName := strings.TrimSuffix(base, filepath.Ext(base))
	ext := filepath.Ext(outName)
	stem := strings.TrimSuffix(outName, ext)
	finalOut, skipped, err := ResolveOutputPath(stem, destDir, ext, conflict)
	if err != nil || skipped {
		return 0, 0, err
	}
	inF, err := os.Open(WinLongPath(archivePath))
	if err != nil {
		return 0, 0, err
	}
	defer inF.Close()
	tmpOut := finalOut + ".archivatore-tmp"
	outF, err := os.OpenFile(WinLongPath(tmpOut), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, 0, err
	}
	cmd := newHiddenCmd(tools.Zstd, "-d", fmt.Sprintf("-T%d", threads), "-c")
	cmd.Stdin = inF
	cmd.Stdout = outF
	rErr := runExternalWithMonitor(cmd, tmpOut, 1, 0, emit)
	_ = outF.Close()
	if rErr != nil {
		_ = os.Remove(WinLongPath(tmpOut))
		return 0, 0, rErr
	}
	if err := os.Rename(WinLongPath(tmpOut), WinLongPath(finalOut)); err != nil {
		_ = os.Remove(WinLongPath(tmpOut))
		return 0, 0, err
	}
	info, _ := os.Stat(WinLongPath(finalOut))
	var sz int64
	if info != nil {
		sz = info.Size()
	}
	return 1, sz, nil
}
