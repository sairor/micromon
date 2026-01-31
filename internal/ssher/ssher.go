package ssher

import (
	"bytes"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHClient struct {
	Config *ssh.ClientConfig
    Host   string
    Port   int
}

func NewSSHClient(user, password, host string, port int) *SSHClient {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Use known_hosts in prod
		Timeout:         5 * time.Second,
	}

    return &SSHClient{
        Config: config,
        Host:   host,
        Port:   port,
    }
}

func (s *SSHClient) RunCommand(cmd string) (string, error) {
    addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	client, err := ssh.Dial("tcp", addr, s.Config)
	if err != nil {
		return "", fmt.Errorf("failed to dial: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	var b bytes.Buffer
	session.Stdout = &b
	if err := session.Run(cmd); err != nil {
		return "", fmt.Errorf("failed to run command: %v", err)
	}

	return b.String(), nil
}
