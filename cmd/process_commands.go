package cmd

import (
	"fmt"
	"github.com/Azure/azure-sdk-for-go/storage"
	"time"
	"io/ioutil"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"regexp"
	"github.com/spf13/cobra"
	log "github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"sync"
)

var (
	accountName string
	accountKey  string
	containerName string
	prefix string
	blobCli     storage.BlobStorageClient
	lastBlob	storage.Blob
	nsgFileRegexp *regexp.Regexp
)

var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Process NSG Files from Azure Blob Storage",
	Run: func(cmd *cobra.Command, args []string) {
		initClient()
		process()
	},
}

func init() {
	nsgFileRegexp = regexp.MustCompile(`.*\/(.*)\/y=([0-9]{4})\/m=([0-9]{2})\/d=([0-9]{2})\/h=([0-9]{2})\/m=([0-9]{2}).*`)
	processCmd.PersistentFlags().StringP("path", "", "/tmp/azlog", "Where to Save the files")
	processCmd.PersistentFlags().StringP("prefix", "", "", "Prefix")
	processCmd.PersistentFlags().StringP("storage_account_name", "", "", "Account")
	processCmd.PersistentFlags().StringP("storage_account_key", "", "", "Key")
	processCmd.PersistentFlags().StringP("container_name", "", "", "Container Name")
	viper.BindPFlag("prefix", processCmd.PersistentFlags().Lookup("prefix"))
	viper.BindPFlag("storage_account_name", processCmd.PersistentFlags().Lookup("storage_account_name"))
	viper.BindPFlag("storage_account_key", processCmd.PersistentFlags().Lookup("storage_account_key"))
	viper.BindPFlag("container_name", processCmd.PersistentFlags().Lookup("container_name"))
	RootCmd.AddCommand(processCmd)
}

func initClient(){
	accountName = viper.GetString("storage_account_name")
	accountKey = viper.GetString("storage_account_key")
	prefix = viper.GetString("prefix")
	client, _ := storage.NewBasicClient(accountName, accountKey)
	containerName = viper.GetString("container_name")
	blobCli = client.GetBlobService()
}

func process() {
	container := blobCli.GetContainerReference(containerName)
	matchingBlobs, err := getBlobList(container)
	if err != nil {
		log.Errorf("Error Loading Blob List - Error %v", err)
		os.Exit(2)
	}
	var wg sync.WaitGroup
	wg.Add(len(matchingBlobs))
	for _, blob := range matchingBlobs {
		go processBlob(blob, &wg)
	}
	wg.Wait()
}

func printBlob(b storage.Blob, wg *sync.WaitGroup){
	defer wg.Done()
	log.Info(b.Name)
}

func processBlob(b storage.Blob, wg *sync.WaitGroup){
	defer wg.Done()
	log.Infof("Processing %v", b.Name)
	nsgLog, err := getNsgLogs(b); if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	alogs := []ArcSightLog{}
	for _, record := range nsgLog.Records {
		for _, flow := range record.Properties.Flows {
			for _, subFlow := range flow.Flows {
				for _, flowTuple := range subFlow.FlowTuples {
					alog := ArcSightLog{}
					tuples := strings.Split(flowTuple,",")
					epochTime, _ := strconv.ParseInt(tuples[0],10,64)
					alog.ResourceID = record.ResourceID
					alog.Time = time.Unix(epochTime, 0)
					alog.SourceIp = tuples[1]
					alog.DestinationIp = tuples[2]
					alog.SourcePort = tuples[3]
					alog.DestinationPort = tuples[4]
					alog.Protocol = tuples[5]
					alog.TrafficFlow = tuples[6]
					alog.Traffic = tuples[7]
					alog.Rule = flow.Rule
					alog.Mac = subFlow.Mac
					alogs = append(alogs, alog)
				}
			}
		}
	}
	outJson, _ := json.Marshal(alogs)
	logName, _ := getAsFileName(b)
	err = ioutil.WriteFile(logName, outJson, 0666)
	log.Infof("Wrote File: %v . Events: %v", logName, len(alogs))
	if err != nil {
		log.Errorf("write file failed: %v", err)
		os.Exit(1)
	}
}

func getAsFileName(b storage.Blob) (string, error){
	bm := nsgFileRegexp.FindStringSubmatch(b.Name)
	if len(bm) == 7 {
		fileName := fmt.Sprintf("nsgLog-%s-%s%s%s%s%s.json", bm[1], bm[2], bm[3], bm[4], bm[5], bm[6])
		return fileName, nil
	}else{
		return "", fmt.Errorf("Error Parsing Blob.Name")
	}
}

func getNsgLogs(b storage.Blob) (NsgLog, error) {
	readCloser, err := b.Get(nil)
	nsgLog := NsgLog{}
	defer readCloser.Close()
	if err != nil {
		return nsgLog, fmt.Errorf("get blob failed: %v", err)
	}
	bytesRead, err := ioutil.ReadAll(readCloser)
	if err != nil {
		return nsgLog, fmt.Errorf("read body failed: %v", err)
	}
	err = json.Unmarshal(bytesRead,&nsgLog)
	if err != nil {
		return nsgLog, fmt.Errorf("json parse body failed: %v", err)
	}
	return nsgLog, nil
}

func getBlobList(cnt *storage.Container) ([]storage.Blob, error) {
	params := storage.ListBlobsParameters{}
	params.Prefix = prefix
	list, err := cnt.ListBlobs(params)
	if err != nil {
		return []storage.Blob{}, err
	}
	return list.Blobs, nil
}
