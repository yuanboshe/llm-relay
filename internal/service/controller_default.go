package service

// DefaultController provides the platform process operations.
type DefaultController struct{}

func (DefaultController) IsRunning(pid int) bool {
	return processIsRunning(pid)
}

func (DefaultController) Start(executable string, args []string, logPath string) (int, error) {
	return startProcess(executable, args, logPath)
}

func (DefaultController) Stop(pid int) error {
	return stopProcess(pid)
}
