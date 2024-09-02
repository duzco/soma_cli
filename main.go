package main

import (
	"encoding/xml"
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"math/cmplx"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"


	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/mjibson/go-dsp/fft"
	"golang.org/x/net/html/charset"
)

type Config struct {
	Stations map[string]Station `yaml:"stations"`
}

type Station struct {
	Display  bool   `yaml:"display"`
	URL      string `yaml:"url"`
	Shortcut string `yaml:"shortcut"`
	SongsURL string `yaml:"songs_url"`
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

var (
	stations       map[string]Station
	stopPlayback = make(chan bool)
	//stopAnalysis = make(chan bool)
	inputBuffer    string
	inputTimeout   = 2 * time.Second // Timeout for flushing the buffer
	lastInputTime  time.Time
	renderLock	   sync.Mutex
)

func main() {
	// Read the config file
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	// Parse the YAML data into the Config struct
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("Failed to parse YAML: %v", err)
	}

	// Assign loaded stations to the global stations variable
	stations = config.Stations
	if err := termui.Init(); err != nil {
		log.Fatalf("Failed to initialize termui: %v", err)
	}
	defer termui.Close()

	// Create a paragraph widget to display track info
	trackInfo := widgets.NewParagraph()
	trackInfo.Title = "Now Playing"
	trackInfo.Text = "No track selected"
	trackInfo.SetRect(0, 0, 65, 12)

	// Create a bar chart widget to display frequency bands
	bc := widgets.NewBarChart()
	bc.Title = ""
	bc.SetRect(0, 12, 65, 22)
	bc.BarWidth = 4
	bc.Labels = []string{"", "", "", "", "", "", "", "", "", ""} // Remove labels
	bc.NumFormatter = func(v float64) string { return "" } // Prevent values from showing
	bc.BarColors = []termui.Color{termui.ColorCyan}    
	termui.Render(trackInfo, bc)

	// Display station options and await user input
	stationDisplay := widgets.NewParagraph()
	stationDisplay.Title = "My Stations"
	stationDisplay.Text = getStationList()
	lines := strings.Count(stationDisplay.Text, "\n") + 2 // Add 2 for padding (title, etc.)
	stationDisplay.SetRect(0, 22, 65, 22 + lines)

	termui.Render(trackInfo, bc, stationDisplay)

	// Start listening for keyboard events
	uiEvents := termui.PollEvents()

	for {
		e := <-uiEvents
		if e.Type == termui.KeyboardEvent {
			now := time.Now()

			// Check if the buffer should be reset
			if now.Sub(lastInputTime) > inputTimeout {
				inputBuffer = ""
			}

			lastInputTime = now
			inputBuffer += e.ID

			// Check if the accumulated input matches any station shortcut
			if stationName, station := getStationByShortcut(inputBuffer); station != nil {				go func() {
					fetchAndPrintTrackData(station.SongsURL, stationName, trackInfo)
					termui.Render(trackInfo)
				}()
				go playStream(station.URL, bc)
				
				// Clear the buffer after successful match
				inputBuffer = ""
			}
		}

		// Handle quitting
		if e.Type == termui.KeyboardEvent && (e.ID == "q" || e.ID == "<C-c>") {
			break
		}
	}
}

func getStationList() string {
	// Extract station names and shortcuts
	stationKeys := make([]string, 0, len(stations))
	for name := range stations {
		stationKeys = append(stationKeys, name)
	}

	// Sort the station names alphabetically
	sort.Strings(stationKeys)

	var builder strings.Builder
	for _, name := range stationKeys {
		station := stations[name]
		if station.Display {
			builder.WriteString(fmt.Sprintf("%s - %s\n", station.Shortcut, name))
		}
	}
	return builder.String()
}

func getStationByShortcut(shortcut string) (string, *Station) {
	for name, station := range stations {
		if shortcut == station.Shortcut {
			return name, &station
		}
	}
	return "", nil
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
	ticker := time.NewTicker(20 * time.Millisecond) // Limit updates to 5 times per second
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
			analyzeFrequencies(leftChannel, bc)
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
	renderLock.Lock()
    defer renderLock.Unlock()
	bc.Data = bands
	termui.Render(bc)
}

func calculateBands(samples []float64) []float64 {
	fftResult := fft.FFTReal(samples)
	numBands := 15
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

func fetchAndPrintTrackData(songsURL string, selectedStationName string, trackInfo *widgets.Paragraph) {
	resp, err := http.Get(songsURL)
	if err != nil {
		trackInfo.Text = fmt.Sprintf("Error fetching track data: %v", err)
		return
	}
	defer resp.Body.Close()

	// Create a new XML decoder with charset support
	decoder := xml.NewDecoder(resp.Body)
	decoder.CharsetReader = charset.NewReaderLabel // Adds support for non-UTF-8 charsets

	var songs Songs
	if err := decoder.Decode(&songs); err != nil {
		trackInfo.Text = fmt.Sprintf("Error parsing track data: %v", err)
		return
	}

	var builder strings.Builder

	// Print the current track (first track)
	if len(songs.Songs) > 0 {
		firstSong := songs.Songs[0]
		builder.WriteString(fmt.Sprintf("Now playing on %s\n", selectedStationName))
		builder.WriteString("Track:                   Artist:               Album:\n")
		builder.WriteString(fmt.Sprintf("%-23.23s  %-20.20s  %-15.15s\n\n", firstSong.Title, firstSong.Artist, firstSong.Album))
	}

	builder.WriteString("  --------------------------History--------------------------\n")
	// Print the other tracks
	for _, song := range songs.Songs[1:6] { // Five most recent tracks
		builder.WriteString(fmt.Sprintf("%-23.23s  %-20.20s  %-15.15s\n", song.Title, song.Artist, song.Album))
	}

	// Update the trackInfo widget with the current and previous tracks
	trackInfo.Text = builder.String()
	termui.Render(trackInfo)
}
