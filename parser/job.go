package parser

import (
	"encoding/json"
	"fmt"
	"github.com/dimitertodorov/nsg-parser/pool"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"path/filepath"
	"sync"
	"time"
	"reflect"
)

type Job struct {
	Name          string
	Options       *JobOptions
	ProcessStatus ProcessStatus
	AzureClient   *AzureClient         `json:"-"`
	ParserClient  NsgParserClient      `json:"-"`
	ResultsChan   chan AzureNsgLogFile `json:"-"`
	DoneChan      chan bool            `json:"-"`
	LogFiles      []*AzureNsgLogFile   `json:"-"`
	Tasks         []*pool.Task         `json:"-"`
	TaskPool      pool.Pool            `json:"-"`
	StartTime     time.Time
	EndTime       time.Time
	processMutex  *sync.Mutex `json:"-"`
	Status        string
}

type JobOptions struct {
	StartRecordTime time.Time
	EndRecordTime   time.Time
	DataPath        string
	Concurrency     int
}

func NewJob(options *JobOptions, processStatus ProcessStatus, azureClient *AzureClient, parserClient NsgParserClient) (*Job, error) {
	job := Job{
		Name:          "nsg-parser",
		Options:       options,
		ProcessStatus: processStatus,
		AzureClient:   azureClient,
		ParserClient:  parserClient,
		processMutex:  &sync.Mutex{},
	}
	job.ResultsChan = make(chan AzureNsgLogFile)
	job.DoneChan = make(chan bool)
	return &job, nil
}

func (job *Job) LoadUnprocessedLogFiles() error {
	matchingBlobs, err := job.AzureClient.GetBlobsByPrefix(job.AzureClient.Prefix)
	if err != nil {
		return err
	}
	for _, blob := range matchingBlobs {
		logFile, err := NewAzureNsgLogFile(blob)
		if err != nil {
			return err
		}
		if logFile.LogTime.After(job.Options.StartRecordTime) {
			lastProcessedFile, ok := job.ProcessStatus[logFile.Blob.Name]
			if ok {
				if logFile.LastModified.After(lastProcessedFile.LastModified) {
					logFile.LastProcessedTimeStamp = lastProcessedFile.LastProcessedTimeStamp
					logFile.LastProcessedRecord = lastProcessedFile.LastProcessedRecord
					logFile.LastProcessedRange = lastProcessedFile.LastProcessedRange
					logFile.Logger().Info("processing modified blob")
					job.LogFiles = append(job.LogFiles, &logFile)
				} else {
					lastProcessedFile.Logger().Debug("skipping unmodified blob")
					continue
				}
			} else {
				logFile.Logger().Info("processing new blob")
				job.LogFiles = append(job.LogFiles, &logFile)
			}
		}
	}
	return nil
}

func (job *Job) LoadTasks() {
	for _, logFile := range job.LogFiles {
		logFile := logFile
		fileTask := pool.NewTask(func() error {
			logFile.Logger().WithField("type", fmt.Sprintf("%T", job.ParserClient)).Info("processing started")
			return job.ParserClient.ProcessNsgLogFile(logFile, job.ResultsChan)
		})
		job.Tasks = append(job.Tasks, fileTask)
	}
}

func (job *Job) Run() {
	job.StartTime = time.Now()
	job.processMutex.Lock()
	job.Status = "RUNNING"
	defer func() {
		job.Complete()

		job.processMutex.Unlock()
	}()
	go job.logFileSink()
	taskPool := pool.NewPool(job.Tasks, 1)
	job.TaskPool = *taskPool
	job.TaskPool.Run()
	for _, task := range job.TaskPool.Tasks {
		if task.Err != nil {
			log.Error(task.Err)
		}
	}
	close(job.ResultsChan)
	<-job.DoneChan
}

func (job *Job) Logger() *log.Entry {
	return log.WithFields(log.Fields{
		"JobName":           job.Name,
		"ClientType": 		reflect.TypeOf(job.ParserClient).String(),
	})
}

func (job *Job)	Complete() {
	job.EndTime = time.Now()
	job.Logger().Infof("job run took %s ", time.Since(job.StartTime))
	job.SaveProcessStatus()
	job.LoadProcessStatus()
	job.LogFiles = []*AzureNsgLogFile{}
	job.Status = "COMPLETE"
}

func (job *Job) LoadProcessStatus() error {
	processStatus, err := ReadProcessStatus(job.Options.DataPath, job.ProcessStatusFileName())
	if err != nil {
		return err
	}
	job.ProcessStatus = processStatus
	return nil
}

func (job *Job) SaveProcessStatus() error {
	outJson, err := json.Marshal(job.ProcessStatus)
	if err != nil {
		return err
	}
	path := filepath.Join(job.Options.DataPath, job.ProcessStatusFileName())
	err = ioutil.WriteFile(path, outJson, 0666)
	return err
}

func (job *Job) ProcessStatusFileName() string {
	return fmt.Sprintf("nsg-parser-status-%s.json", job.Name)
}

func (job *Job) logFileSink() {
	for {
		processedFile, more := <-job.ResultsChan
		if more {
			processedFile.Logger().Info("processing completed")
			job.ProcessStatus[processedFile.Name] = &processedFile
		} else {
			job.DoneChan <- true
			return
		}
	}
}
