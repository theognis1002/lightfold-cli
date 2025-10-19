package firewall

import "lightfold/pkg/ssh"

// Manager defines the interface for firewall management
type Manager interface {
	// OpenPort opens a port in the firewall
	OpenPort(port int) error

	// ClosePort closes a port in the firewall
	ClosePort(port int) error

	// IsPortOpen checks if a port is open
	IsPortOpen(port int) (bool, error)

	// ListOpenPorts returns all open ports
	ListOpenPorts() ([]int, error)

	// Name returns the name of the firewall manager
	Name() string

	// IsAvailable checks if the firewall is available on the system
	IsAvailable() (bool, error)
}

// ManagerFactory is a function that creates a new Manager instance
type ManagerFactory func(executor *ssh.Executor) Manager

var (
	registry = make(map[string]ManagerFactory)
)

// Register registers a new firewall manager factory with the given name
func Register(name string, factory ManagerFactory) {
	registry[name] = factory
}

// GetManager retrieves a firewall manager by name
func GetManager(name string, executor *ssh.Executor) (Manager, error) {
	factory, exists := registry[name]
	if !exists {
		// Default to UFW
		factory = registry["ufw"]
	}

	return factory(executor), nil
}

// GetDefault returns the default firewall manager (UFW)
func GetDefault(executor *ssh.Executor) Manager {
	mgr, _ := GetManager("ufw", executor)
	return mgr
}
