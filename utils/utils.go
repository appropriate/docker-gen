package utils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// based off of https://github.com/dotcloud/docker/blob/2a711d16e05b69328f2636f88f8eac035477f7e4/utils/utils.go
func ParseHost(addr string) (string, string, error) {

	var (
		proto string
		host  string
		port  int
	)
	addr = strings.TrimSpace(addr)
	switch {
	case addr == "tcp://":
		return "", "", fmt.Errorf("Invalid bind address format: %s", addr)
	case strings.HasPrefix(addr, "unix://"):
		proto = "unix"
		addr = strings.TrimPrefix(addr, "unix://")
		if addr == "" {
			addr = "/var/run/docker.sock"
		}
	case strings.HasPrefix(addr, "tcp://"):
		proto = "tcp"
		addr = strings.TrimPrefix(addr, "tcp://")
	case strings.HasPrefix(addr, "fd://"):
		return "fd", addr, nil
	case addr == "":
		proto = "unix"
		addr = "/var/run/docker.sock"
	default:
		if strings.Contains(addr, "://") {
			return "", "", fmt.Errorf("Invalid bind address protocol: %s", addr)
		}
		proto = "tcp"
	}

	if proto != "unix" && strings.Contains(addr, ":") {
		hostParts := strings.Split(addr, ":")
		if len(hostParts) != 2 {
			return "", "", fmt.Errorf("Invalid bind address format: %s", addr)
		}
		if hostParts[0] != "" {
			host = hostParts[0]
		} else {
			host = "127.0.0.1"
		}

		if p, err := strconv.Atoi(hostParts[1]); err == nil && p != 0 {
			port = p
		} else {
			return "", "", fmt.Errorf("Invalid bind address format: %s", addr)
		}

	} else if proto == "tcp" && !strings.Contains(addr, ":") {
		return "", "", fmt.Errorf("Invalid bind address format: %s", addr)
	} else {
		host = addr
	}
	if proto == "unix" {
		return proto, host, nil

	}
	return proto, fmt.Sprintf("%s:%d", host, port), nil
}

func GetEndpoint() (string, error) {
	defaultEndpoint := "unix:///var/run/docker.sock"
	if os.Getenv("DOCKER_HOST") != "" {
		defaultEndpoint = os.Getenv("DOCKER_HOST")
	}

	/*
		if endpoint != "" {
			defaultEndpoint = endpoint
		}
	*/

	_, _, err := ParseHost(defaultEndpoint)
	if err != nil {
		return "", err
	}

	return defaultEndpoint, nil
}

// SplitKeyValueSlice takes a string slice where values are of the form
// KEY, KEY=, KEY=VALUE  or KEY=NESTED_KEY=VALUE2, and returns a map[string]string where items
// are split at their first `=`.
func SplitKeyValueSlice(in []string) map[string]string {
	env := make(map[string]string)
	for _, entry := range in {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			parts = append(parts, "")
		}
		env[parts[0]] = parts[1]
	}
	return env

}

// PathExists returns whether the given file or directory exists or not
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func isBlank(str string) bool {
	for _, r := range str {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func RemoveBlankLines(reader io.Reader, writer io.Writer) {
	breader := bufio.NewReader(reader)
	bwriter := bufio.NewWriter(writer)

	for {
		line, err := breader.ReadString('\n')

		if !isBlank(line) {
			bwriter.WriteString(line)
		}

		if err != nil {
			break
		}
	}

	bwriter.Flush()
}
