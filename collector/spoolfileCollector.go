package collector

import (
	"griesbacher.org/nagflux/logging"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"
)

const (
	WorkSuffix               = ".working"
	MinFileAgeInSeconds      = time.Duration(60) * time.Second
	IntervalToCheckDirectory = time.Duration(5) * time.Second
)

type SpoolfileCollector struct {
	quit           chan bool
	jobs           chan string
	spoolDirectory string
	workers        []*SpoolfileWorker
}

func SpoolfileCollectorFactory(spoolDirectory string, workerAmount int, results chan interface{}, fieldseperator, charToReplaceSpace string) *SpoolfileCollector {
	s := &SpoolfileCollector{make(chan bool), make(chan string, 100), spoolDirectory, make([]*SpoolfileWorker, workerAmount)}

	gen := SpoolfileWorkerGenerator(s.jobs, results, fieldseperator, charToReplaceSpace)

	for w := 0; w < workerAmount; w++ {
		s.workers[w] = gen()
	}

	go s.run()
	return s
}

func (s *SpoolfileCollector) Stop() {
	s.quit <- true
	<-s.quit
	for _, worker := range s.workers {
		worker.Stop()
	}
	logging.GetLogger().Debug("SpoolfileCollector stopped")
}

func (s *SpoolfileCollector) run() {
	for {
		select {
		case <-s.quit:
			s.quit <- true
			return
		case <-time.After(IntervalToCheckDirectory):
			files, _ := ioutil.ReadDir(s.spoolDirectory)
			for _, currentFile := range files {
				if !isInUse(currentFile) && isItTime(currentFile.ModTime(), MinFileAgeInSeconds) {
					currentPath := path.Join(s.spoolDirectory, currentFile.Name())
					select {
					case <-s.quit:
						s.quit <- true
						return
					case s.jobs <- currentPath:
					}
				}
			}
		}
	}
}

func isInUse(file os.FileInfo) bool {
	return strings.HasSuffix(file.Name(), WorkSuffix)
}
func isItTime(timeStamp time.Time, duration time.Duration) bool {
	return time.Now().After(timeStamp.Add(duration))
}