package ssher

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Pool manages SSH connections
type Pool struct {
	mu      sync.Mutex
	clients map[string]*ssh.Client
}

var (
	poolInstance *Pool
	poolOnce     sync.Once
)

// GetPool returns the singleton instance of the SSH pool
func GetPool() *Pool {
	poolOnce.Do(func() {
		poolInstance = &Pool{
			clients: make(map[string]*ssh.Client),
		}
	})
	return poolInstance
}

// GetClient retrieves or creates an SSH client for the given target
func (p *Pool) GetClient(user, password, host string, port int, useSSHKey bool) (*ssh.Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	authMod := "pass"
	if useSSHKey {
		authMod = "key"
	}
	key := fmt.Sprintf("%s@%s:%d[%s]", user, host, port, authMod)

	// Check if client exists and is active (simple check)
	if client, ok := p.clients[key]; ok {
		// Verify if session can be opened (ping)
		_, _, err := client.Conn.SendRequest("keepalive@openssh.com", true, nil)
		if err == nil {
			return client, nil
		}
		// active connection closed/broken, remove and recreate
		client.Close()
		delete(p.clients, key)
	}

	// Create new client auth methods
	var auth []ssh.AuthMethod
	if useSSHKey {
		keyPath := filepath.Join("data", "id_rsa")
		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key: %v", err)
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %v", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	} else {
		auth = append(auth, ssh.Password(password))
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
		Config: ssh.Config{
			KeyExchanges: []string{
				"diffie-hellman-group-exchange-sha256",
				"diffie-hellman-group14-sha256",
				"diffie-hellman-group14-sha1",
				"diffie-hellman-group1-sha1",
				"curve25519-sha256@libssh.org",
			},
			Ciphers: []string{
				"aes128-ctr", "aes192-ctr", "aes256-ctr",
				"aes128-gcm@openssh.com", "chacha20-poly1305@openssh.com",
				"aes128-cbc", "3des-cbc",
			},
		},
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}

	p.clients[key] = client
	return client, nil
}

// RunCommand executes a command on the remote host using the pool
func (p *Pool) RunCommand(user, password, host string, port int, useSSHKey bool, cmd string) (string, error) {
	client, err := p.GetClient(user, password, host, port, useSSHKey)
	if err != nil {
		return "", err
	}

	session, err := client.NewSession()
	if err != nil {
		// If session creation fails, maybe the connection is dead?
		// Try invalidating the connection and retry once
		p.mu.Lock()
		authMod := "pass"
		if useSSHKey {
			authMod = "key"
		}
		key := fmt.Sprintf("%s@%s:%d[%s]", user, host, port, authMod)
		if c, ok := p.clients[key]; ok && c == client {
			c.Close()
			delete(p.clients, key)
		}
		p.mu.Unlock()

		// Retry once
		client, err = p.GetClient(user, password, host, port, useSSHKey)
		if err != nil {
			return "", fmt.Errorf("retry dial failed: %v", err)
		}
		session, err = client.NewSession()
		if err != nil {
			return "", fmt.Errorf("retry session failed: %v", err)
		}
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	if err := session.Run(cmd); err != nil {
		// Log stderr if available
		log.Printf("SSH Command Warning: %v. Stderr: %s", err, stderrBuf.String())
		return stdoutBuf.String(), err
	}

	return stdoutBuf.String(), nil
}

// DownloadFile fetches a file from remote host to local path
func (p *Pool) DownloadFile(user, password, host string, port int, useSSHKey bool, remotePath, localPath string) error {
	client, err := p.GetClient(user, password, host, port, useSSHKey)
	if err != nil {
		return err
	}

	// Create SFTP client
	from, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("failed to create sftp client: %v", err)
	}
	defer from.Close()

	// Open remote file
	srcFile, err := from.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file %s: %v", remotePath, err)
	}
	defer srcFile.Close()

	// Create local directory if not exists
	err = os.MkdirAll(filepath.Dir(localPath), 0755)
	if err != nil {
		return fmt.Errorf("failed to create local directory: %v", err)
	}

	// Create local file
	dstFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file %s: %v", localPath, err)
	}
	defer dstFile.Close()

	// Copy content
	_, err = srcFile.WriteTo(dstFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %v", err)
	}

	return nil
}
