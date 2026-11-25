/* MAP
 * §1 package & imports   L7-L16
 * §2 FileEntry & Scan    L18-L94
 */

// §1 package & imports
package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// §2 FileEntry & Scan
type FileEntry struct {
	AbsPath string
	RelPath string
	Size    int64
	Mode    fs.FileMode
	ModTime int64
	IsDir   bool
}

func ScanSource(srcPath string) (entries []FileEntry, fileCount int64, totalBytes int64, err error) {
	cleanSrc := filepath.Clean(srcPath)
	info, err := os.Lstat(WinLongPath(cleanSrc))
	if err != nil {
		return nil, 0, 0, err
	}
	rootName := filepath.Base(cleanSrc)

	if !info.IsDir() {
		entries = append(entries, FileEntry{
			AbsPath: cleanSrc,
			RelPath: filepath.ToSlash(rootName),
			Size:    info.Size(),
			Mode:    info.Mode(),
			ModTime: info.ModTime().Unix(),
			IsDir:   false,
		})
		return entries, 1, info.Size(), nil
	}

	err = filepath.WalkDir(WinLongPath(cleanSrc), func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		rel, rErr := filepath.Rel(WinLongPath(cleanSrc), path)
		if rErr != nil {
			return rErr
		}
		var archiveRel string
		if rel == "." {
			archiveRel = rootName
		} else {
			archiveRel = rootName + "/" + filepath.ToSlash(rel)
		}
		archiveRel = strings.TrimPrefix(archiveRel, "./")

		dInfo, iErr := d.Info()
		if iErr != nil {
			return iErr
		}
		isDir := d.IsDir()
		size := int64(0)
		if !isDir {
			size = dInfo.Size()
			fileCount++
			totalBytes += size
		}
		cleanOrig := cleanSrc
		if rel != "." {
			cleanOrig = filepath.Join(cleanSrc, rel)
		}
		entries = append(entries, FileEntry{
			AbsPath: cleanOrig,
			RelPath: archiveRel,
			Size:    size,
			Mode:    dInfo.Mode(),
			ModTime: dInfo.ModTime().Unix(),
			IsDir:   isDir,
		})
		return nil
	})
	return entries, fileCount, totalBytes, err
}
