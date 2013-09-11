package bugsnag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

const (
	apiEndpoint = "notify.bugsnag.com"
	VERSION     = "0.0.1"
)

var (
	defaultInfo = &notifierInfo{Name: "Bugsnag Go", Version: VERSION, Url: "https://github.com/pettyjamesm/bugsnag-go"}
)

type notifierInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Url     string `json:"url"`
}

type BugsnagContext interface {
	Name() string
	SetUserId(userId string)
	Notify(err error)
}

type BugsnagNotifier interface {
	Notify(err error)
	SetReleaseStage(releaseStage string)
	SetNotifyStages(notifyStages []string)
	SetUseSSL(useSSL bool)
	NewContext(contextName string) BugsnagContext
	SetMaxStackSize(maxSize uint)
}

func NewBugsnagNotifier(apiKey string) BugsnagNotifier {
	notifier := &restNotifier{
		apiKey:       apiKey,
		info:         defaultInfo,
		releaseStage: "production",
		notifyStages: []string{"production"},
		useSSL:       false,
		stackSize:    50,
		queue:        make(chan *bugsnagNotification, 10),
	}
	notifier.invalidateWillNotify()
	go notifier.processQueue()
	return notifier
}

type restNotifier struct {
	apiKey string
	info   *notifierInfo
	//	Release Stage Information
	releaseStage string
	notifyStages []string
	//	Indicates whether the current releaseStage is in notifyStages or not
	willNotify bool
	//	Indicates SSL connections should be used
	useSSL bool
	//	Maximum stack trace size
	stackSize uint
	//	Send queue
	queue chan *bugsnagNotification
}

func (notifier *restNotifier) String() string {
	return fmt.Sprintf("BugsnagNotifier(%v)", *notifier)
}

func (notifier *restNotifier) Notify(err error) {
	notifier.notify(err, nil)
}

func (notifier *restNotifier) notify(err error, context *notifierContext) {
	if !notifier.willNotify {
		return
	}
	exception := bugsnagException{
		ErrorClass: getErrorTypeName(err),
		Message:    err.Error(),
		StackTrace: getStackFrames(2, int(notifier.stackSize)),
	}
	event := bugsnagEvent{
		ReleaseStage: notifier.releaseStage,
		Exceptions:   []bugsnagException{exception},
	}
	if context != nil {
		event.UserId = context.userId
		event.Context = context.name
	}
	notifier.queue <- &bugsnagNotification{
		ApiKey:       notifier.apiKey,
		NotifierInfo: notifier.info,
		Events:       []bugsnagEvent{event},
	}
}

func (notifier *restNotifier) SetReleaseStage(releaseStage string) {
	notifier.releaseStage = releaseStage
	notifier.invalidateWillNotify()
}

func (notifier *restNotifier) SetNotifyStages(releaseStages []string) {
	notifier.notifyStages = releaseStages
	notifier.invalidateWillNotify()
}

func (notifier *restNotifier) invalidateWillNotify() {
	for _, check := range notifier.notifyStages {
		if check == notifier.releaseStage {
			notifier.willNotify = true
			return
		}
	}
	notifier.willNotify = false
}

func (notifier *restNotifier) SetUseSSL(useSSL bool) {
	notifier.useSSL = useSSL
}

func (notifier *restNotifier) NewContext(contextName string) BugsnagContext {
	return &notifierContext{notifier: notifier, name: contextName}
}

func (notifier *restNotifier) SetMaxStackSize(maxSize uint) {
	notifier.stackSize = maxSize
}

func (notifier *restNotifier) processQueue() {
	client := &http.Client{}
	for {
		notification := <-notifier.queue
		notifier.dispatchSingle(client, notification)
	}
}

func (notifier *restNotifier) dispatchSingle(client *http.Client, notification *bugsnagNotification) {
	defer func() {
		if err := recover(); err != nil {
			log.Panicf("Failed to send bugsnag notification!\n\t%s\n", err)
		}
	}()
	var (
		url        string
		serialized []byte
		response   *http.Response
		err        error
	)
	if notifier.useSSL {
		url = "https://" + apiEndpoint
	} else {
		url = "http://" + apiEndpoint
	}

	serialized, err = json.Marshal(notification)
	if err != nil {
		panic(err)
	}

	response, err = client.Post(url, "application/json", bytes.NewReader(serialized))
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case 200:
		//	Successful dispatch, yay
		return
	case 400:
		//	Something wrong with our JSON formatting
		log.Printf("Invalid JSON Sent to Bugsnag: %s\n", string(serialized))
	case 401:
		//	Invalid API Key
		log.Printf("API Key '%s' is not a valid Bugsnag API Key!\n", notifier.apiKey)
	case 413:
		panic(fmt.Errorf("Bugsnag Rejected Notification due to Size (Payload: %d bytes)", len(serialized)))
	case 429:
		log.Printf("Bugsnag Rate-Limit Exceeded")
	default:
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Printf("Unknown Bugsnag Response: %s\n", response.Status)
		} else {
			log.Printf("Unknown Bugsnag Response: %s\n%s\n", response.Status, body)
		}
	}
}

type notifierContext struct {
	notifier *restNotifier
	userId   string
	name     string
}

func (context *notifierContext) Name() string {
	return context.name
}

func (context *notifierContext) Notify(err error) {
	context.notifier.notify(err, context)
}

func (context *notifierContext) SetUserId(userId string) {
	context.userId = userId
}
