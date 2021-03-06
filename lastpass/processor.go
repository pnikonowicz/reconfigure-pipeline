package lastpass

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"code.cloudfoundry.org/commandrunner"
	"gopkg.in/yaml.v2"
)

type Processor struct {
	commandRunner   commandrunner.CommandRunner
	credentialCache map[string]string
}

func NewProcessor(commandRunner commandrunner.CommandRunner) *Processor {
	return &Processor{
		commandRunner:   commandRunner,
		credentialCache: map[string]string{},
	}
}

func (l *Processor) Process(config string) string {
	re := regexp.MustCompile(`\(\((.*)\)\)`)

	processedConfig := re.ReplaceAllStringFunc(config, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		return l.handle(submatches[1])
	})

	return processedConfig
}

func (l *Processor) handle(credHandle string) string {
	pathParts := strings.Split(credHandle, "/")

	credential := l.getCredential(pathParts[0], pathParts[1])

	fragment := ""
	if len(pathParts) > 2 {
		fragment = pathParts[2]
	}

	var encoded []byte

	if fragment != "" {
		// Assume YAML contents, return element
		fragmentMap := map[string]interface{}{}
		err := yaml.Unmarshal([]byte(credential), &fragmentMap)
		if err != nil {
			log.Fatalln(err)
		}

		value, found := fragmentMap[fragment]
		if !found {
			log.Fatalf("could not find key '%s'\n", fragment)
		}

		encoded, _ = json.Marshal(value)
	} else {
		encoded, _ = json.Marshal(credential)
	}

	return string(encoded)
}

func (l *Processor) getCredential(credential, field string) string {
	cacheKey := strings.Join([]string{credential, field}, "/")
	credentialValue := l.credentialCache[cacheKey]

	if credentialValue == "" {
		credentialValue = l.getCredentialFromLastPass(credential, field)
		l.credentialCache[cacheKey] = credentialValue
	}

	return credentialValue
}

func (l *Processor) getCredentialFromLastPass(credential, field string) string {
	fieldFlagMap := map[string]string{
		"Password": "--password",
		"Username": "--username",
		"URL":      "--url",
		"Notes":    "--notes",
	}

	fieldFlag := fieldFlagMap[field]
	if fieldFlag == "" {
		fieldFlag = "--field=" + field
	}

	output := &bytes.Buffer{}

	cmd := exec.Command("lpass", "show", fieldFlag, credential)

	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = output

	err := l.commandRunner.Run(cmd)
	if err != nil {
		log.Fatal(err)
	}

	return strings.TrimSpace(output.String())
}
