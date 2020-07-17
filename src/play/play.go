package play

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/olekukonko/tablewriter"
	log "github.com/schollz/logger"
	"github.com/schollz/s4/src/midi"
	"github.com/schollz/s4/src/music"
	"github.com/schollz/s4/src/sequencer"
)

func PrintDevices() (err error) {
	devices, err := midi.Init()
	if err != nil {
		return
	}
	if len(devices) == 0 {
		err = fmt.Errorf("no devices detected, try plugging some")
		return
	}
	data := [][]string{}
	for _, device := range devices {
		data = append(data, []string{device})
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"INSTRUMENTS"})
	for _, v := range data {
		table.Append(v)
	}
	table.Render()
	return
}

func Play(s4File string) (err error) {
	// show devices
	err = PrintDevices()
	if err != nil {
		return
	}

	if len(s4File) == 0 {
		return
	}

	// start sequencer with midi equipped
	seq := sequencer.New(func(s string, c music.Chord) {
		log.Tracef("[%s] forwarding emit", s)
		errMidi := midi.Midi(s, c)
		if errMidi != nil {
			log.Error(errMidi)
		}
	})

	// shutdown everything on Ctl+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			log.Debug(sig)
			log.Info("shutting down")
			go seq.Stop()
			time.Sleep(500 * time.Millisecond)
			go midi.Shutdown()
			time.Sleep(500 * time.Millisecond)
			os.Exit(1)
		}
	}()

	// load s4 file
	err = seq.Parse(s4File)
	if err != nil {
		return
	}

	// hot-reload file
	go func() {
		err = hotReloadFile(seq, s4File)
		if err != nil {
			log.Error(err)
		}
	}()

	log.Info("playing")
	seq.Start()
	time.Sleep(5 * time.Hour)

	return
}

func hotReloadFile(seq *sequencer.Sequencer, fname string) (err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer watcher.Close()

	lastEvent := time.Now()
	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Debugf("event: %+v", event)
				if event.Op&fsnotify.Write == fsnotify.Write && time.Since(lastEvent).Seconds() > 1 {
					lastEvent = time.Now()
					log.Infof("reloading: %+v", event.Name)
					time.Sleep(100 * time.Millisecond)
					err = seq.Parse(fname)
					if err != nil {
						log.Warnf("problem hot-reloading %s: %s", fname, err.Error())
					} else {
						midi.NotesOff()
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error(err)
			}
		}
	}()

	err = watcher.Add(fname)
	if err != nil {
		return
	}
	<-done
	return
}
