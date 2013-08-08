package main

import (
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

var ProtocolRegexp = regexp.MustCompile("^([a-z]+://)")

type config struct {
	applications map[string]string
	lookup       map[string]string
	patterns     map[*regexp.Regexp]string
}

func loadConfig(file string) (*config, error) {
	conf := &config{
		applications: make(map[string]string),
		lookup:       make(map[string]string),
		patterns:     make(map[*regexp.Regexp]string),
	}

	var v map[string]interface{}
	_, err := toml.DecodeFile(file, &v)
	if err != nil {
		return nil, err
	}

	for group, m := range v {
		m := m.(map[string]interface{})

		switch group {
		case "applications":
			for name, path := range m {
				path, ok := path.(string)
				if !ok {
					return nil, errors.New("Configuration values must be strings")
				}

				conf.applications[name] = path
			}
		case "protocols", "mimes", "extensions":
			for name, app := range m {
				app, ok := app.(string)
				if !ok {
					return nil, errors.New("Configuration values must be strings")
				}

				conf.lookup[name] = app
			}
		case "patterns":
			for pattern, app := range m {
				app, ok := app.(string)
				if !ok {
					return nil, errors.New("Configuration values must be strings")
				}

				rx, err := regexp.Compile(pattern)
				if err != nil {
					return nil, err
				}

				conf.patterns[rx] = app
			}
		}
	}

	return conf, nil
}

func (config *config) findAppName(file string) (string, bool) {
	if protocol := ProtocolRegexp.FindString(file); len(protocol) > 0 {
		appName, ok := config.lookup[protocol]
		return appName, ok
	}

	if mime, err := detectMime(file); err == nil {
		if appName, ok := config.lookup[mime]; ok {
			return appName, true
		}
	}

	if ext := path.Ext(file); len(ext) > 0 {
		if appName, ok := config.lookup[ext]; ok {
			return appName, true
		}
	}

	for pattern, appName := range config.patterns {
		if pattern.MatchString(file) {
			return appName, true
		}
	}

	return "", false
}

func (config *config) resolveAppName(appName string) string {
	switch appName {
	case "browser", "editor":
		app := os.Getenv(strings.ToUpper(appName))
		if len(app) > 0 {
			return app + " $0"
		}

		fallthrough
	default:
		if app, ok := config.applications[appName]; ok {
			return app
		}

		return appName
	}
}

func detectMime(file string) (string, error) {
	cmd := exec.Command("/usr/bin/file", "--mime-type", "-b", file)
	output, err := cmd.Output()
	return strings.TrimSpace(string(output)), err
}

func launch(file, app string) error {
	fmt.Fprintln(os.Stderr, app)
	cmd := exec.Command("/bin/sh", "-c", app+" &", file)
	return cmd.Run()
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "%s <file | url>", os.Args[0])
		os.Exit(1)
	}

	file := os.Args[1]

	config, err := loadConfig(os.ExpandEnv("${HOME}/.no-xdg-open"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not load ~/.no-xdg-open:", err)
		os.Exit(4)
	}

	appName, ok := config.findAppName(file)
	if !ok {
		fmt.Fprintln(os.Stderr, "Couldn't find a way to open", file)
		os.Exit(4)
	}

	app := config.resolveAppName(appName)
	err = launch(file, app)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(4)
	}
}
