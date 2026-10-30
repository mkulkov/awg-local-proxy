//go:build !windows

package coreclient

import (
	"errors"
	"os"
	"os/exec"
)

type realJob struct{}

func newRealJob() (*realJob, error) {
	return nil, errors.New("managed real core requires Windows Job Objects")
}
func (*realJob) assign(*os.Process) error {
	return errors.New("managed real core requires Windows Job Objects")
}
func (*realJob) close() error        { return nil }
func configureRealCommand(*exec.Cmd) {}
