package bugsnag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
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

type Context interface {
	Name() string
	SetUserId(userId string)
	Notify(err interface{})
	NotifyOnPanic(swallowPanic bool)
}

type Notifier interface {
	Notify(err interface{})
	SetReleaseStage(releaseStage string)
	SetNotifyStages(notifyStages []string)
	SetUseSSL(useSSL bool)
	NewContext(contextName string) Context
	SetMaxStackSize(maxSize uint)
	NotifyOnPanic(swallowPanic bool)
	WillNotify() bool
	SentFailureCount() uint64
	UnsentFailureCount() uint64
}

func NewNotifier(apiKey string) Notifier {
	notifier := &restNotifier{
		apiKey:         apiKey,
		info:           defaultInfo,
		releaseStage:   "production",
		notifyStages:   []string{"production"},
		useSSL:         false,
		stackSize:      50,
		httpClient:     &http.Client{},
		queue:          make(chan *bugsnagNotification, 10),
		totalTriggered: uint64(0),
		totalNotified:  uint64(0),
	}
	notifier.invalidateWillNotify()
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
	//	Http Client
	httpClient *http.Client
	//	Send queue
	queue chan *bugsnagNotification
	//	Counters for Sent / Success
	totalTriggered uint64
	totalNotified  uint64
}

func (notifier *restNotifier) String() string {
	return fmt.Sprintf("BugsnagNotifier(%v)", *notifier)
}

func (notifier *restNotifier) SentFailureCount() uint64 {
	return notifier.totalNotified
}

func (notifier *restNotifier) UnsentFailureCount() uint64 {
	return notifier.totalTriggered - notifier.totalNotified
}

func (notifier *restNotifier) WillNotify() bool {
	return notifier.willNotify
}

func (notifier *restNotifier) NotifyOnPanic(swallowPanic bool) {
	if err := recover(); err != nil {
		notifier.notify(err, nil, !swallowPanic)
		if !swallowPanic {
			panic(err)
		}
	}
}

func (notifier *restNotifier) Notify(err interface{}) {
	notifier.notify(err, nil, false)
}

type errorType interface {
	Error() string
}

type stringType interface {
	String() string
}

func (notifier *restNotifier) notify(err interface{}, context *notifierContext, synchronous bool) {
	notifier.totalTriggered++
	if !notifier.willNotify {
		return
	}

	var message string

	switch err.(type) {
	case errorType:
		message = err.(errorType).Error()
	case stringType:
		message = err.(stringType).String()
	default:
		message = fmt.Sprintf("%+v", err)
	}

	exception := bugsnagException{
		ErrorClass: getErrorTypeName(err),
		Message:    message,
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
	notification := &bugsnagNotification{
		ApiKey:       notifier.apiKey,
		NotifierInfo: notifier.info,
		Events:       []bugsnagEvent{event},
	}
	if synchronous {
		notifier.dispatchSingle(notification)
	} else {
		notifier.queue <- notification
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
	result := false
	if notifier.apiKey != "" {
		for _, check := range notifier.notifyStages {
			if check == notifier.releaseStage {
				result = true
				break
			}
		}
	}
	if result && !notifier.willNotify {
		notifier.willNotify = result
		go notifier.processQueue()
	} else if !result && notifier.willNotify {
		notifier.willNotify = result
		notifier.queue <- nil
	}
}

func (notifier *restNotifier) SetUseSSL(useSSL bool) {
	notifier.useSSL = useSSL
}

func (notifier *restNotifier) NewContext(contextName string) Context {
	return &notifierContext{notifier: notifier, name: contextName}
}

func (notifier *restNotifier) SetMaxStackSize(maxSize uint) {
	notifier.stackSize = maxSize
}

func (notifier *restNotifier) processQueue() {
	for notifier.willNotify {
		notification := <-notifier.queue
		if notifier.willNotify && notification != nil {
			notifier.dispatchSingle(notification)
		}
	}
	//	Drain the channel if not notifying
	for !notifier.willNotify {
		select {
		case _ = <-notifier.queue:
			continue
		default:
			break
		}
	}
}

func (notifier *restNotifier) dispatchSingle(notification *bugsnagNotification) {
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

	response, err = notifier.httpClient.Post(url, "application/json", bytes.NewReader(serialized))
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case 200:
		//	Successful dispatch, yay
		notifier.totalNotified++
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
		time.Sleep(time.Millisecond * 10)
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

func (context *notifierContext) Notify(err interface{}) {
	context.notifier.notify(err, context, false)
}

func (context *notifierContext) NotifyOnPanic(swallowPanic bool) {
	if err := recover(); err != nil {
		context.notifier.notify(err, context, !swallowPanic)
		if !swallowPanic {
			panic(err)
		}
	}
}

func (context *notifierContext) SetUserId(userId string) {
	context.userId = userId
}
