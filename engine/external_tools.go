/* MAP
 * §1 package & imports            L7-L19
 * §2 7z / RAR / XZ / BZ2 compress L21-L97
 * §3 External extract & roots     L99-L212
 */

// §1 package & imports
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// §2 7z / RAR / XZ / BZ2 compress
func runTarRepackage(tools ToolPaths, tmpOut string, tarArgs []string, entries []FileEntry, totalFiles, totalBytes int64, emit *Emitter) error {
	outF, oErr := os.OpenFile(WinLongPath(tmpOut), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if oErr != nil {
		return oErr
	}
	cmd := newHiddenCmd(tools.Tar, tarArgs...)
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
	tarErr := WriteTarToWriter(stdin, entries, 4, totalFiles, totalBytes, emit)
	_ = stdin.Close()
	waitErr := cmd.Wait()
	closeErr := outF.Close()
	if waitErr != nil {
		return fmt.Errorf("tar.exe failed: %s", strings.TrimSpace(stderr.String()))
	}
	if tarErr != nil {
		return tarErr
	}
	return closeErr
}

func CompressSevenZip(tools ToolPaths, srcPath, tmpOut string, entries []FileEntry, level, threads int, totalFiles, totalBytes int64, emit *Emitter) error {
	if tools.SevenZip != "" {
		cmd := newHiddenCmd(tools.SevenZip, "a", "-t7z", fmt.Sprintf("-mx=%d", level), fmt.Sprintf("-mmt=%d", threads), "-y", tmpOut, srcPath)
		return runExternalWithMonitor(cmd, tmpOut, totalFiles, totalBytes, emit)
	}
	if tools.Tar == "" {
		return errors.New("neither 7z.exe nor tar.exe found")
	}
	opts := fmt.Sprintf("compression-level=%d", level)
	return runTarRepackage(tools, tmpOut, []string{"--format=7zip", "--options=" + opts, "-cf", "-", "@-"}, entries, totalFiles, totalBytes, emit)
}

func CompressRar(tools ToolPaths, srcPath, tmpOut string, level, threads int, totalFiles, totalBytes int64, emit *Emitter) error {
	if tools.Rar == "" {
		return errors.New("WinRAR Rar.exe not installed")
	}
	rLvl := mapRarLevel(level)
	cmd := newHiddenCmd(tools.Rar, "a", "-ma5", "-ep1", "-r",
		"-m"+strconv.Itoa(rLvl),
		"-mt"+strconv.Itoa(threads),
		"-idq", "-y", "--", tmpOut, filepath.Base(srcPath),
	)
	cmd.Dir = filepath.Dir(srcPath)
	return runExternalWithMonitor(cmd, tmpOut, totalFiles, totalBytes, emit)
}

func CompressTarXzOrBz2(tools ToolPaths, format, tmpOut string, entries []FileEntry, level, threads int, totalFiles, totalBytes int64, emit *Emitter) error {
	if tools.Tar == "" {
		return errors.New("tar.exe not found")
	}
	if level < 1 {
		level = 1
	} else if level > 9 {
		level = 9
	}
	// Write UTF-8 PAX TAR to ASCII temp file, then raw-compress via bsdtar so non-ASCII paths stay 100% intact
	tmpTarF, err := os.CreateTemp("", "arctar-*.tar")
	if err != nil {
		return err
	}
	tmpTarPath := tmpTarF.Name()
	_ = tmpTarF.Close()
	defer os.Remove(tmpTarPath)

	if err := CompressCapsuleTar(tmpTarPath, entries, threads, totalFiles, totalBytes, emit); err != nil {
		return err
	}
	outF, oErr := os.OpenFile(WinLongPath(tmpOut), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if oErr != nil {
		return oErr
	}
	var args []string
	if format == "tar-xz" {
		opts := fmt.Sprintf("xz:compression-level=%d,xz:threads=%d", level, threads)
		args = []string{"--format=raw", "-cJf", "-", "--options=" + opts, tmpTarPath}
	} else {
		opts := fmt.Sprintf("bzip2:compression-level=%d", level)
		args = []string{"--format=raw", "-cjf", "-", "--options=" + opts, tmpTarPath}
	}
	cmd := newHiddenCmd(tools.Tar, args...)
	cmd.Stdout = outF
	rErr := runExternalWithMonitor(cmd, tmpOut, totalFiles, totalBytes, emit)
	cErr := outF.Close()
	if rErr != nil {
		return rErr
	}
	return cErr
}

// §3 External extract & roots
func ExtractWithExternalTool(tools ToolPaths, format, archivePath, destDir string, threads int, emit *Emitter) (int64, int64, error) {
	if err := os.MkdirAll(WinLongPath(destDir), 0755); err != nil {
		return 0, 0, err
	}
	info, _ := os.Stat(WinLongPath(archivePath))
	var sz int64
	if info != nil {
		sz = info.Size()
	}

	var cmd *exec.Cmd
	var cleanup func()
	switch format {
	case "rar":
		if tools.Rar == "" {
			return 0, 0, errors.New("WinRAR Rar.exe not installed")
		}
		destSlash := strings.TrimRight(destDir, `\/`) + `\`
		cmd = newHiddenCmd(tools.Rar, "x", "-o+", "-mt"+strconv.Itoa(threads), "-idq", "-y", "--", archivePath, destSlash)
	case "7z":
		if tools.SevenZip != "" {
			cmd = newHiddenCmd(tools.SevenZip, "x", "-y", fmt.Sprintf("-mmt=%d", threads), "-o"+destDir, archivePath)
		} else if tools.Tar != "" {
			tmpArc, clean, lErr := linkOrTempASCII(archivePath, ".7z")
			if lErr != nil {
				return 0, 0, lErr
			}
			cleanup = clean
			cmd = newHiddenCmd(tools.Tar, "-xf", tmpArc, "-C", ".")
			cmd.Dir = destDir
		} else {
			return 0, 0, errors.New("no 7z extractor found")
		}
	case "tar-xz", "tar-bz2":
		if tools.Tar == "" {
			return 0, 0, errors.New("tar.exe not found")
		}
		ext := ".tar.xz"
		if format == "tar-bz2" {
			ext = ".tar.bz2"
		}
		tmpArc, clean, lErr := linkOrTempASCII(archivePath, ext)
		if lErr != nil {
			return 0, 0, lErr
		}
		cleanup = clean
		cmd = newHiddenCmd(tools.Tar, "-xf", tmpArc, "-C", ".")
		cmd.Dir = destDir
	default:
		return 0, 0, errors.New("unsupported external format: " + format)
	}
	if cleanup != nil {
		defer cleanup()
	}
	if err := runExternalWithMonitor(cmd, "", 0, sz, emit); err != nil {
		return 0, 0, err
	}
	var files, bytesOut int64
	_ = filepath.WalkDir(WinLongPath(destDir), func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files++
			if fi, fErr := d.Info(); fErr == nil {
				bytesOut += fi.Size()
			}
		}
		return nil
	})
	return files, bytesOut, nil
}

func ListExternalRoots(tools ToolPaths, format, archivePath string) ([]string, error) {
	var cmd *exec.Cmd
	var cleanup func()
	if format == "rar" && tools.Rar != "" {
		cmd = newHiddenCmd(tools.Rar, "lb", "--", archivePath)
	} else if tools.Tar != "" {
		ext := ".7z"
		if format == "tar-xz" {
			ext = ".tar.xz"
		} else if format == "tar-bz2" {
			ext = ".tar.bz2"
		}
		tmpArc, clean, lErr := linkOrTempASCII(archivePath, ext)
		if lErr != nil {
			return nil, lErr
		}
		cleanup = clean
		cmd = newHiddenCmd(tools.Tar, "-tf", tmpArc)
	} else {
		return nil, errors.New("cannot inspect archive roots")
	}
	if cleanup != nil {
		defer cleanup()
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var roots []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.ReplaceAll(line, `\`, "/"), "/"))
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "/", 2)
		if parts[0] != "" && parts[0] != "." && !seen[parts[0]] {
			seen[parts[0]] = true
			roots = append(roots, parts[0])
		}
	}
	return roots, nil
}
