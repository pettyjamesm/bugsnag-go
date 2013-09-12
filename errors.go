package bugsnag

import (
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
	InProject  bool   `json:"inProject,omitempty"`
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

func (e *bugsnagException) getFileContext() string {
	if len(e.StackTrace) > 0 {
		trace := &e.StackTrace[0]
		return fmt.Sprintf("%s#%s,L%d", trace.File, trace.Method, trace.LineNumber)
	} else {
		return ""
	}
}

func getErrorTypeName(err interface{}) string {
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

	for i := 0; i < depth; i++ {
		pc := callers[i]
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			break
		} else if fn.Name() == "runtime.panic" {
			continue
		}

		var trimmed string
		filename, line := fn.FileLine(pc)

		filename, trimmed = simplifyFilePath(filename)

		var inProject bool
		if strings.HasSuffix(trimmed, "/src/") || strings.HasSuffix(trimmed, "/pkg/") {
			inProject = false
		} else {
			inProject = true
		}

		if len(filename) > 1 && filename[0] == '/' {
			filename = filename[1:]
		}

		output[written] = stacktraceFrame{File: filename, LineNumber: uint(line), Method: fn.Name(), InProject: inProject}
		written++
	}

	return output[:written]
}

var goroot string
var sourcePaths []string

func init() {
	goroot = runtime.GOROOT() + "/src/pkg/"
	sourcePaths = strings.Split(os.Getenv("GOPATH"), ":")
}

func simplifyFilePath(path string) (string, string) {
	pathLen := len(path)
	if strings.HasPrefix(path, goroot) && pathLen > len(goroot) {
		return path[len(goroot):], goroot
	}
	for _, check := range sourcePaths {
		if strings.HasPrefix(path, check) && pathLen > len(check) {
			var src, pkg string
			src = path + "/src/"
			pkg = path + "/pkg/"
			if strings.HasPrefix(path, src) && pathLen > len(src) {
				return path[len(src):], src
			} else if strings.HasPrefix(path, pkg) && pathLen > len(pkg) {
				return path[len(pkg):], pkg
			} else {
				return path[len(check):], check
			}
		}
	}
	return path, ""
}
