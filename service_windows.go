//go:build windows

package main

import "golang.org/x/sys/windows/svc"

// serviceName 必须与安装脚本里 New-Service 使用的服务名一致。
const serviceName = "sql-relay"

func runningAsService() bool {
	ok, err := svc.IsWindowsService()
	return err == nil && ok
}

func runService(start func(stop <-chan struct{})) {
	_ = svc.Run(serviceName, &serviceHandler{start: start})
}

type serviceHandler struct {
	start func(stop <-chan struct{})
}

func (h *serviceHandler) Execute(args []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		h.start(stop)
		close(done)
	}()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case req := <-requests:
			switch req.Cmd {
			case svc.Interrogate:
				status <- req.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				close(stop)
				<-done
				return false, 0
			}
		case <-done:
			return false, 0
		}
	}
}
