// License: GPLv3 Copyright: 2025, Kovid Goyal, <kovid at kovidgoyal.net>

//go:build windows

package shm

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var _ = fmt.Print

// Windows has no POSIX shared memory. Shared memory objects are ordinary
// files in a well known directory (the same one used by kitty's C shm_open()
// shim), memory mapped via CreateFileMapping/MapViewOfFile.
var SHM_DIR = filepath.Join(os.TempDir(), "kitty-shm")

const SHM_NAME_MAX = 30
const SHM_REQUIRED_PREFIX = ""

type mapping struct {
	handle windows.Handle
	size   int
}

var mappings_lock sync.Mutex
var mappings = map[uintptr]mapping{}

func mmap(sz int, access AccessFlags, fd int, off int64) ([]byte, error) {
	if sz <= 0 {
		return nil, fmt.Errorf("cannot mmap a region of size: %d", sz)
	}
	var protect, desired uint32
	switch access {
	case READ:
		protect, desired = windows.PAGE_READONLY, windows.FILE_MAP_READ
	case WRITE:
		protect, desired = windows.PAGE_READWRITE, windows.FILE_MAP_READ|windows.FILE_MAP_WRITE
	case COPY:
		protect, desired = windows.PAGE_WRITECOPY, windows.FILE_MAP_COPY
	}
	maxsz := uint64(off) + uint64(sz)
	h, err := windows.CreateFileMapping(windows.Handle(fd), nil, protect, uint32(maxsz>>32), uint32(maxsz&0xffffffff), nil)
	if err != nil {
		return nil, err
	}
	addr, err := windows.MapViewOfFile(h, desired, uint32(uint64(off)>>32), uint32(uint64(off)&0xffffffff), uintptr(sz))
	if err != nil {
		windows.CloseHandle(h)
		return nil, err
	}
	mappings_lock.Lock()
	mappings[addr] = mapping{handle: h, size: sz}
	mappings_lock.Unlock()
	return unsafe.Slice((*byte)(unsafe.Pointer(addr)), sz), nil
}

func munmap(s []byte) error {
	if len(s) == 0 {
		return nil
	}
	addr := uintptr(unsafe.Pointer(unsafe.SliceData(s)))
	mappings_lock.Lock()
	m, found := mappings[addr]
	delete(mappings, addr)
	mappings_lock.Unlock()
	if !found {
		return errors.New("munmap: address was not mapped by this package")
	}
	err := windows.UnmapViewOfFile(addr)
	if cerr := windows.CloseHandle(m.handle); err == nil {
		err = cerr
	}
	return err
}

func truncate_or_unlink(ans *os.File, size uint64, unlink func(string) error) (err error) {
	if err = ans.Truncate(int64(size)); err != nil {
		_ = ans.Close()
		_ = unlink(ans.Name())
		return fmt.Errorf("Failed to truncate SHM file %s to size: %d with error: %w", ans.Name(), size, err)
	}
	return
}

type file_based_mmap struct {
	f            *os.File
	pos          int64
	region       []byte
	unlinked     bool
	special_name string
}

func file_path_from_name(name string) string {
	name = strings.TrimPrefix(name, "/")
	return filepath.Join(SHM_DIR, name)
}

// Files are opened with FILE_SHARE_DELETE so that they can be unlinked while
// still mapped by other processes, as with POSIX shm
func open_shared(path string, access, createmode uint32) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, createmode, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}

type file_disposition_info struct {
	DeleteFile uint8
}

type file_disposition_info_ex struct {
	Flags uint32
}

// Remove the name immediately even if the file is still open elsewhere
func unlink_posix(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(p, windows.DELETE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return &os.PathError{Op: "remove", Path: path, Err: err}
	}
	defer windows.CloseHandle(h)
	dex := file_disposition_info_ex{Flags: windows.FILE_DISPOSITION_DELETE | windows.FILE_DISPOSITION_POSIX_SEMANTICS}
	err = windows.SetFileInformationByHandle(h, windows.FileDispositionInfoEx, (*byte)(unsafe.Pointer(&dex)), uint32(unsafe.Sizeof(dex)))
	if err != nil { // filesystem without POSIX delete support
		d := file_disposition_info{DeleteFile: 1}
		err = windows.SetFileInformationByHandle(h, windows.FileDispositionInfo, (*byte)(unsafe.Pointer(&d)), uint32(unsafe.Sizeof(d)))
	}
	if err != nil {
		return &os.PathError{Op: "remove", Path: path, Err: err}
	}
	return nil
}

func ShmUnlink(name string) error {
	return unlink_posix(file_path_from_name(name))
}

func file_mmap(f *os.File, size uint64, access AccessFlags, truncate bool, special_name string) (MMap, error) {
	if truncate {
		if err := truncate_or_unlink(f, size, unlink_posix); err != nil {
			return nil, err
		}
	}
	region, err := mmap(int(size), access, int(f.Fd()), 0)
	if err != nil {
		f.Close()
		unlink_posix(f.Name())
		return nil, err
	}
	return &file_based_mmap{f: f, region: region, special_name: special_name}, nil
}

func (self *file_based_mmap) Seek(offset int64, whence int) (ret int64, err error) {
	switch whence {
	case io.SeekStart:
		self.pos = offset
	case io.SeekEnd:
		self.pos = int64(len(self.region)) + offset
	case io.SeekCurrent:
		self.pos += offset
	}
	return self.pos, nil
}

func (self *file_based_mmap) Read(b []byte) (n int, err error) {
	return Read(self, b)
}

func (self *file_based_mmap) Write(b []byte) (n int, err error) {
	return Write(self, b)
}

func (self *file_based_mmap) Stat() (fs.FileInfo, error) {
	return self.f.Stat()
}

func (self *file_based_mmap) Name() string {
	if self.special_name != "" {
		return self.special_name
	}
	return filepath.Base(self.f.Name())
}

func (self *file_based_mmap) Flush() error {
	if len(self.region) == 0 {
		return nil
	}
	return windows.FlushViewOfFile(uintptr(unsafe.Pointer(unsafe.SliceData(self.region))), uintptr(len(self.region)))
}

func (self *file_based_mmap) FileSystemName() string {
	return self.f.Name()
}

func (self *file_based_mmap) Slice() []byte {
	return self.region
}

func (self *file_based_mmap) Close() (err error) {
	if self.region != nil {
		err = munmap(self.region)
		self.region = nil
		// On Windows a file cannot be deleted while a view of it is mapped,
		// so close the file only after unmapping.
		self.f.Close()
	}
	return err
}

func (self *file_based_mmap) Unlink() (err error) {
	if self.unlinked {
		return nil
	}
	self.unlinked = true
	return unlink_posix(self.f.Name())
}

func (self *file_based_mmap) IsFileSystemBacked() bool { return true }

func create_temp(pattern string, size uint64) (ans MMap, err error) {
	var prefix, suffix string
	prefix, suffix, err = prefix_and_suffix(pattern)
	if err != nil {
		return
	}
	if err = os.MkdirAll(SHM_DIR, 0o700); err != nil {
		return nil, &ErrNotSupported{err: err}
	}
	var f *os.File
	try := 0
	for {
		name := prefix + RandomFilename() + suffix
		f, err = open_shared(file_path_from_name(name), windows.GENERIC_READ|windows.GENERIC_WRITE, windows.CREATE_NEW)
		if err != nil {
			if errors.Is(err, fs.ErrExist) {
				try += 1
				if try > 10000 {
					return nil, &os.PathError{Op: "createtemp", Path: prefix + "*" + suffix, Err: fs.ErrExist}
				}
				continue
			}
			return
		}
		break
	}
	return file_mmap(f, size, WRITE, true, "")
}

func Open(name string, size uint64) (MMap, error) {
	ans, err := open_shared(file_path_from_name(name), windows.GENERIC_READ, windows.OPEN_EXISTING)
	if err != nil {
		return nil, err
	}
	if size == 0 {
		s, err := ans.Stat()
		if err != nil {
			ans.Close()
			return nil, fmt.Errorf("Failed to stat SHM file with error: %w", err)
		}
		size = uint64(s.Size())
	}
	return file_mmap(ans, size, READ, false, name)
}
