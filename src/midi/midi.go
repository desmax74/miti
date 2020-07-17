package midi

import (
	"fmt"

	"github.com/schollz/idim/src/music"
	log "github.com/schollz/logger"
	"github.com/schollz/portmidi"
)

var outputChannels map[string]chan music.Chord
var inited bool

func Init() (devices []string, err error) {
	defer func() {
		if err == nil {
			inited = true
		}
	}()
	err = portmidi.Initialize()
	if err != nil {
		return
	}
	log.Debugf("found %d devices", portmidi.CountDevices())

	outputChannels = make(map[string]chan music.Chord)
	for i := 0; i < portmidi.CountDevices(); i++ {
		di := portmidi.Info(portmidi.DeviceID(i))
		log.Debugf("device %d: '%s', i/o: %v/%v", i, di.Name, di.IsInputAvailable, di.IsOutputAvailable)
		if di.IsOutputAvailable {
			devices = append(devices, di.Name)
			var outStream *portmidi.Stream
			outStream, err = portmidi.NewOutputStream(portmidi.DeviceID(i), 4096, 0)
			if err != nil {
				return
			}
			// create a buffered channel for each instrument
			outputChannels[di.Name] = make(chan music.Chord, 100)
			// create a go-routine for each instrument
			go func(instrument string, outputStream *portmidi.Stream) {
				midis := make([]int64, 100)
				velocities := make([]int64, 100)
				notesOn := make(map[int64]bool)
				for {
					chord := <-outputChannels[instrument]
					// special things
					// midi note -1 turns off all on notes
					// midi note -2 turns off all on notes and shuts down
					if chord.Notes[0].MIDI < 0 {
						// turn off all notes
						for note := range notesOn {
							if notesOn[note] {
								outputStream.WriteShort(0x80, note, 0)
							}
						}
					}
					if chord.Notes[0].MIDI == -2 {
						// shutdown
						outputStream.Close()
						return
					}
					lenChordNotes := 0
					for i, n := range chord.Notes {
						midis[i] = int64(n.MIDI)
						if onState, ok := notesOn[midis[i]]; ok {
							if onState && chord.On {
								// this note already has this state
								continue
							}
						}
						notesOn[midis[i]] = chord.On
						velocities[i] = 100
						lenChordNotes++
					}
					if chord.On {
						err = outputStream.WriteShorts(0x90, midis[:lenChordNotes], velocities[:lenChordNotes])
					} else {
						err = outputStream.WriteShorts(0x80, midis[:lenChordNotes], velocities[:lenChordNotes])
					}
					if err != nil {
						log.Error(err)
					}
				}
			}(di.Name, outStream)
			if err != nil {
				err = fmt.Errorf("could not get output from: '%s'", di.Name)
				return
			}
		}
	}

	return
}

func Shutdown() (err error) {
	inited = false
	for out := range outputChannels {
		outputChannels[out] <- music.Chord{Notes: []music.Note{music.Note{MIDI: -2}}, On: false}
	}
	return portmidi.Terminate()
}

func NotesOff() (err error) {
	for out := range outputChannels {
		outputChannels[out] <- music.Chord{Notes: []music.Note{music.Note{MIDI: -1}}, On: false}
	}
	return
}

func Midi(msg string, chord music.Chord) (err error) {
	log.Trace("got emit")
	if !inited {
		err = fmt.Errorf("not initialized")
		return
	}
	if len(chord.Notes) == 0 {
		return
	}
	if _, ok := outputChannels[msg]; !ok {
		err = fmt.Errorf("no such device: %s", msg)
		return
	}
	outputChannels[msg] <- chord
	// log.Trace("building midi")
	// midis := make([]int64, len(chord.Notes))
	// velocities := make([]int64, len(chord.Notes))
	// for i, n := range chord.Notes {
	// 	midis[i] = int64(n.MIDI)
	// 	encounteredNotes[midis[i]] = struct{}{}
	// 	velocities[i] = 100
	// }
	// log.Trace("sending midi")
	// if chord.On {
	// 	log.Tracef("[%s] %+v", msg, midis)
	// 	err = outputStreams[msg].WriteShorts(0x90, midis, velocities)
	// } else {
	// 	err = outputStreams[msg].WriteShorts(0x80, midis, velocities)
	// }
	// log.Trace("sent")
	// if err != nil {
	// 	return
	// }
	return
}
