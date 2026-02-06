package notify

import (
	"fmt"
	"os/exec"
	"runtime"
)

type Notifier struct{}

func New() *Notifier {
	return &Notifier{}
}

func (n *Notifier) Send(title, body string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("notify-send", title, body).Run()
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		return exec.Command("osascript", "-e", script).Run()
	case "windows":
		return fmt.Errorf("native Windows notification backend is not implemented yet")
	case "android":
		return fmt.Errorf("native Android notification backend is not implemented yet")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
