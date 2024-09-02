package main

import (
	//"bufio"
	"encoding/xml"
	"fmt"
	"log"
	"math/cmplx"
	"net/http"
	//"os"
	"strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/mjibson/go-dsp/fft"
	"golang.org/x/net/html/charset"
)

type Station struct {
	URL      string
	Shortcut rune
	SongsURL string // URL for fetching song data
}

// Song structure to map the XML data
type Song struct {
	Title  string `xml:"title"`
	Artist string `xml:"artist"`
	Album  string `xml:"album"`
}

// Songs struct to wrap a list of Song elements
type Songs struct {
	Songs []Song `xml:"song"`
}

// Station data: station name -> stream URL and Songs URL
var stations = map[string]Station{
	"Lush":           {URL: "https://ice6.somafm.com/lush-128-mp3", Shortcut: 'l', SongsURL: "https://somafm.com/songs/lush.xml"},
	"Groove Salad":   {URL: "https://ice6.somafm.com/groovesalad-128-mp3", Shortcut: 'g', SongsURL: "https://somafm.com/songs/groovesalad.xml"},
	"Indie Pop Rocks!": {URL: "https://ice6.somafm.com/indiepop-128-mp3", Shortcut: 'i', SongsURL: "https://somafm.com/songs/indiepop.xml"},
	"Secret Agent":   {URL: "https://ice6.somafm.com/secretagent-128-mp3", Shortcut: 's', SongsURL: "https://somafm.com/songs/secretagent.xml"},
	"Underground 80s": {URL: "https://ice6.somafm.com/u80s-128-mp3", Shortcut: '8', SongsURL: "https://somafm.com/songs/u80s.xml"},
}

var (
	stopPlayback = make(chan bool)
	stopAnalysis = make(chan bool)
)

func main() {
	if err := termui.Init(); err != nil {
		log.Fatalf("Failed to initialize termui: %v", err)
	}
	defer termui.Close()

	// Create a paragraph widget to display track info
	trackInfo := widgets.NewParagraph()
	trackInfo.Title = "Now Playing"
	trackInfo.Text = "No track selected"
	trackInfo.SetRect(0, 0, 50, 5)

	// Create a bar chart widget to display frequency bands
	bc := widgets.NewBarChart()
	bc.Title = "Frequency Bands"
	bc.SetRect(0, 5, 50, 15)
	bc.Labels = []string{"", "", "", "", "", "", "", "", "", ""} // Remove labels
	bc.NumFormatter = func(v float64) string { return "" } // Prevent values from showing
	termui.Render(trackInfo, bc)

	// Display station options and await user input
	stationDisplay := widgets.NewParagraph()
	stationDisplay.Title = "Available Stations"
	stationDisplay.Text = getStationList()
	stationDisplay.SetRect(0, 15, 50, 25)

	termui.Render(trackInfo, bc, stationDisplay)

	// Start listening for keyboard events
	uiEvents := termui.PollEvents()

	for {
		e := <-uiEvents
		if e.Type == termui.KeyboardEvent {
			switch e.ID {
			case "q", "<C-c>":
				return
			default:
				// Check if the pressed key matches any station shortcut
				if station := getStationByShortcut(e.ID); station != nil {
					go func() {
						fetchAndPrintTrackData(station.SongsURL, trackInfo)
						termui.Render(trackInfo)
					}()
					go playStream(station.URL, bc)
				}
			}
		}
	}
}

func getStationList() string {
	var builder strings.Builder
	builder.WriteString("Press a key to select a station:\n\n")
	for name, station := range stations {
		builder.WriteString(fmt.Sprintf("%c -- %s\n", station.Shortcut, name))
	}
	return builder.String()
}

func getStationByShortcut(shortcut string) *Station {
	for _, station := range stations {
		if shortcut == string(station.Shortcut) {
			return &station
		}
	}
	return nil
}

func playStream(streamURL string, bc *widgets.BarChart) {
	resp, err := http.Get(streamURL)
	if err != nil {
		log.Fatalf("Failed to get stream: %v", err)
	}
	defer resp.Body.Close()

	streamer, format, err := mp3.Decode(resp.Body)
	if err != nil {
		log.Fatalf("Failed to decode mp3: %v", err)
	}
	defer streamer.Close()

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	done := make(chan bool)
	ticker := time.NewTicker(200 * time.Millisecond) // Limit updates to 5 times per second
	defer ticker.Stop()

	// Create a custom streamer that analyzes the frequencies
	streamerWithAnalysis := beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		n, ok = streamer.Stream(samples)
		if !ok {
			return n, ok
		}

		
		// Analyze the left channel
		leftChannel := make([]float64, n)
		for i := 0; i < n; i++ {
			leftChannel[i] = samples[i][0]
		}

		select {
		case <-ticker.C: // Only analyze and update at the specified interval
			go analyzeFrequencies(leftChannel, bc)
		default:
		}

		return n, ok
	})

	speaker.Play(beep.Seq(streamerWithAnalysis, beep.Callback(func() {
		done <- true
	})))

	// Wait for playback to finish or for a stop signal
	select {
	case <-done:
	case <-stopPlayback:
		speaker.Clear()
	}
}

func analyzeFrequencies(leftChannel []float64, bc *widgets.BarChart) {
	bands := calculateBands(leftChannel)


	// Eliminate the first and last bands
	if len(bands) > 2 {
		bands = bands[1 : len(bands)-1]
	}

	//mu.Lock()
	//defer mu.Unlock()
	bc.Data = bands
	termui.Render(bc)
}

func calculateBands(samples []float64) []float64 {
	fftResult := fft.FFTReal(samples)
	numBands := 12
	bandWidth := len(fftResult) / numBands
	bands := make([]float64, numBands)

	for i := 0; i < numBands; i++ {
		bandPower := 0.0
		for j := 0; j < bandWidth; j++ {
			bandPower += cmplx.Abs(fftResult[i*bandWidth+j])
		}
		bands[i] = bandPower / float64(bandWidth)
	}

	return bands
}

func fetchAndPrintTrackData(songsURL string, trackInfo *widgets.Paragraph) {
	resp, err := http.Get(songsURL)
	if err != nil {
		fmt.Println("Error fetching track data:", err)
		return
	}
	defer resp.Body.Close()

	// Create a new XML decoder with charset support
	decoder := xml.NewDecoder(resp.Body)
	decoder.CharsetReader = charset.NewReaderLabel // Adds support for non-UTF-8 charsets

	var songs Songs
	if err := decoder.Decode(&songs); err != nil {
		trackInfo.Text = "Error parsing track data"
		return
	}

	if len(songs.Songs) > 0 {
		firstSong := songs.Songs[0]
		trackInfo.Text = fmt.Sprintf("Now playing: %s - %s\nAlbum: %s", firstSong.Title, firstSong.Artist, firstSong.Album)
	} else {
		trackInfo.Text = "No track info available"
	}
}