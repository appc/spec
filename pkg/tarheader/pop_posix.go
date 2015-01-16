package tarheader

import (
	"archive/tar"
	"os"
	"syscall"
)

func init() {
	populateHeaderStat = append(populateHeaderStat, populateHeaderUnix)
}

func populateHeaderUnix(h *tar.Header, fi os.FileInfo, seen map[uint64]string) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}
	h.Uid = int(st.Uid)
	h.Gid = int(st.Gid)
	if st.Mode&syscall.S_IFMT == syscall.S_IFBLK || st.Mode&syscall.S_IFMT == syscall.S_IFCHR {
		// FIXME: it would be better to use the C macros: major(3), minor(3)
		h.Devminor = int64(st.Rdev & 0xff)
		h.Devmajor = int64((st.Rdev >> 8) & 0xfff)
	}
	// If we have already seen this inode, generate a hardlink
	p, ok := seen[uint64(st.Ino)]
	if ok {
		h.Linkname = p
		h.Typeflag = tar.TypeLink
	} else {
		seen[uint64(st.Ino)] = h.Name
	}
}
