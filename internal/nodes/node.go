package nodes

type ExitNode struct {
	ID           string
	Host         string
	Port         int
	Authorized   bool
	Healthy      bool
	Region       string
	NetworkLabel string
	KillSwitch   bool
}

func New(host string, port int) ExitNode {
	return ExitNode{
		ID:         "node-1",
		Host:       host,
		Port:       port,
		Authorized: true,
		Healthy:    true,
	}
}
