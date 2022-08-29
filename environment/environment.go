package environment

import (
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

func RunCommand(command string, args ...string) ([]string, error) {
	logrus.Infof("Running command %s with args %s", command, args)

	cmd := exec.Command(command, args...)

	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Infof("Output Error: %s", out)
		return []string{}, err
	}

	lines := strings.Split(string(out), "\n")

	logrus.Infof("Output: %s", lines)
	return lines, err
}
