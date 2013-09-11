package bugsnag

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
)

type stacktraceFrame struct {
	File       string `json:"file"`
	LineNumber uint   `json:"lineNumber"`
	Method     string `json:"method"`
}

type bugsnagException struct {
	ErrorClass string            `json:"errorClass"`
	Message    string            `json:"message"`
	StackTrace []stacktraceFrame `json:"stacktrace"`
}

type bugsnagEvent struct {
	UserId       string             `json:"userId,omitempty"`
	AppVersion   string             `json:"appVersion,omitempty"`
	OsVersion    string             `json:"osVersion,omitempty"`
	ReleaseStage string             `json:"releaseStage"`
	Context      string             `json:"context,omitempty"`
	GroupingHash string             `json:"groupingHash,omitempty"`
	Exceptions   []bugsnagException `json:"exceptions"`
}

type bugsnagNotification struct {
	ApiKey       string         `json:"apiKey"`
	NotifierInfo *notifierInfo  `json:"notifier"`
	Events       []bugsnagEvent `json:"events"`
}

func (e *bugsnagEvent) String() string {
	if bytes, error := json.Marshal(e); error != nil {
		panic(error)
	} else {
		return string(bytes)
	}
}

func getErrorTypeName(err error) string {
	errorType := reflect.TypeOf(err)
	if errorType.Kind() == reflect.Ptr {
		errorType = errorType.Elem()
	}
	return fmt.Sprintf("%s:%s", errorType.PkgPath(), errorType.Name())
}

func getStackFrames(skipFrames, maxFrames int) []stacktraceFrame {
	if skipFrames < 0 {
		skipFrames = 0
	}

	callers := make([]uintptr, maxFrames)
	depth := runtime.Callers(skipFrames+2, callers)
	output := make([]stacktraceFrame, depth)

	written := 0

	sourcePaths := getSourcePaths()

	for i := 0; i < depth; i++ {
		pc := callers[i]
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			break
		}
		filename, line := fn.FileLine(pc)

		filename = simplifyFilePath(sourcePaths, filename)

		output[i] = stacktraceFrame{File: filename, LineNumber: uint(line), Method: fn.Name()}
		written++
	}

	return output[:written]
}

func simplifyFilePath(prefixes []string, path string) string {
	for _, check := range prefixes {
		if strings.HasPrefix(path, check) && len(path) > len(check) {
			return path[len(check):]
		}
	}
	return path
}

func getSourcePaths() []string {
	goroot := runtime.GOROOT() + "/src/pkg/"
	gopath := strings.Split(os.Getenv("GOPATH"), ":")
	output := make([]string, len(gopath)+1)
	output[0] = goroot
	for i, path := range gopath {
		output[i+1] = path + "/src/"
	}
	return output
}
