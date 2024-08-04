package container

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// sendFDs sends file descriptors over a Unix domain socket.
func sendFDs(sock int, fds []int) error {
	buf := []byte{0}
	iov := syscall.Iovec{Base: &buf[0], Len: 1}

	cmsg := unix.UnixRights(fds...)
	msg := syscall.Msghdr{
		Iov:        &iov,
		Iovlen:     1,
		Control:    &cmsg[0],
		Controllen: uint64(len(cmsg)),
	}

	n, _, errno := syscall.Syscall(syscall.SYS_SENDMSG, uintptr(sock), uintptr(unsafe.Pointer(&msg)), 0)
	if n != 1 {
		return fmt.Errorf("sendmsg failed: %v", errno)
	}
	return nil
}

// sendRootFD connects to a Unix domain socket and sends file descriptors.
func sendRootFD(sockPath string, chrootFD, memFD int) (int, error) {
	sock, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return -1, fmt.Errorf("socket creation failed: %v", err)
	}
	defer syscall.Close(sock)

	addr := syscall.SockaddrUnix{Name: sockPath}
	if err := syscall.Connect(sock, &addr); err != nil {
		return -1, fmt.Errorf("connect failed: %v", err)
	}

	fmt.Printf("send chrootFD=%d\n", chrootFD)
	fds := []int{chrootFD, memFD}
	if err := sendFDs(sock, fds); err != nil {
		return -1, err
	}

	var status int
	if _, err := syscall.Read(sock, (*[4]byte)(unsafe.Pointer(&status))[:]); err != nil {
		return -1, fmt.Errorf("read failed: %v", err)
	}

	return status, nil
}

// forkRequest sends the namespace file descriptors for the targetPid process
// and the passed package list to a lambda server listening on the Unix socket at sockPath.
func (c *Container) forkRequest(rootDir *os.File, memCG *os.File) error {
	status, err := sendRootFD(c.commsSock(), int(rootDir.Fd()), int(memCG.Fd()))
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("received non-zero status: %d", status)
	}
	return nil
}
