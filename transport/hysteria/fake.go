package hysteria

import "syscall"

var _ syscall.RawConn = (*FakeSyscallConn)(nil)

type FakeSyscallConn struct{}

func (c *FakeSyscallConn) Control(f func(fd uintptr)) error {
	return nil
}

func (c *FakeSyscallConn) Read(f func(fd uintptr) (done bool)) error {
	return nil
}

func (c *FakeSyscallConn) Write(f func(fd uintptr) (done bool)) error {
	return nil
}
